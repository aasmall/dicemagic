package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"cloud.google.com/go/logging"
	"github.com/aasmall/dicemagic/lib/envreader"
	"github.com/aasmall/dicemagic/lib/handler"
	log "github.com/aasmall/dicemagic/lib/logger"
	"github.com/go-redis/redis/v7"
	"github.com/gorilla/mux"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//MAXSLOTS is the total number of slots a redis cluster has
const MAXSLOTS = 16384

type clusterConfiguratorConfig struct {
	projectID            string
	debug                bool
	local                bool
	logname              string
	podname              string
	redisStatefulsetName string
	redisPort            string
	redisNamespace       string
	redisLabelSelector   string
	localPodIP           string
}
type clusterConfigurator struct {
	k8sClient   *kubernetes.Clientset
	redisClient *redis.ClusterClient
	localClient *redis.Client
	log         *log.Logger
	config      *clusterConfiguratorConfig
	nodes       clusterNodes
}
type clusterNode struct {
	IPAddress               string
	ID                      string
	isPrimary               bool
	replicaOf               *clusterNode
	primaryFor              *clusterNode
	replicated              bool
	podName                 string
	currentReplicationState string
	currentPrimaryID        string
}
type clusterNodes []*clusterNode

func (nodes clusterNodes) String() string {
	s := "["
	for i, node := range nodes {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%+v", node)
	}
	return s + "]"
}

type clusterSlots []redis.ClusterSlot

type reshardCommands []*reshardCommand
type reshardCommand struct {
	order      int
	nodeToID   string
	nodeFromID string
	count      int
}
type nodeReshardTargets []*nodeReshardTarget
type nodeReshardTarget struct {
	node              *clusterNode
	nodeOffset        int
	assignedSlotCount int
	targetSlotCount   int
}

func main() {
	ctx := context.Background()
	mu := &sync.Mutex{}
	// Gather environment variables
	configReader := new(envreader.EnvReader)
	config := &clusterConfiguratorConfig{
		projectID:            configReader.GetEnv("PROJECT_ID"),
		debug:                configReader.GetEnvBool("DEBUG"),
		local:                configReader.GetEnvBool("LOCAL"),
		logname:              configReader.GetEnv("LOG_NAME"),
		podname:              configReader.GetEnv("POD_NAME"),
		redisPort:            configReader.GetEnv("REDIS_PORT"),
		redisStatefulsetName: configReader.GetEnv("REDIS_STATEFULSET_NAME"),
		redisNamespace:       configReader.GetEnv("REDIS_NAMESPACE"),
		redisLabelSelector:   configReader.GetEnv("REDIS_LABEL_SELECTOR"),
		localPodIP:           configReader.GetEnv("LOCAL_POD_IP"),
	}
	if configReader.Errors {
		log.Fatalf("could not gather environment variables. Failed variables: %v", configReader.MissingKeys)
	}
	var cc = &clusterConfigurator{config: config}

	// Instantiate logger
	cc.log = log.New(
		cc.config.projectID,
		log.WithDefaultSeverity(logging.Error),
		log.WithDebug(cc.config.debug),
		log.WithLogName(cc.config.logname),
		log.WithPrefix(cc.config.podname+": "),
	)

	// create simple webserver for healthchecks
	r := mux.NewRouter()
	r.Handle("/healthz", handler.Handler{Env: cc, H: healtzHandler})
	r.Handle("/readyz", handler.Handler{Env: cc, H: readyzHandler})

	// Define a server with timeouts
	srv := &http.Server{
		Addr:         ":8888",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Printf("ListenAndServe error: %+v", err)
			panic(err)
		}
	}()

	// Create new Kubernetes Client
	client, err := newKubernetesClient()
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}
	cc.k8sClient = client

	// if this is the first run, create a redis-client.
	var redisClientURIs []string
	for _, n := range *cc.getClusterNodes(ctx, false) {
		redisClientURIs = append(redisClientURIs, n.IPAddress+":"+cc.config.redisPort)
	}
	redisClientURIs = append(redisClientURIs, "localhost:"+cc.config.redisPort)
	cc.log.Debug("Creating redis client on first init")
	redisClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    redisClientURIs,
		Password: "",
	})
	localClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:" + cc.config.redisPort, // use default Addr
		Password: "",                                 // no password set
		DB:       0,                                  // use default DB
	})
	cc.localClient = localClient
	cc.redisClient = redisClient
	cc.log.Debugf("Got redis client from %v", redisClientURIs)

	// keep config up to date. Re-create redis client from k8s state if borked.
	go func(mu *sync.Mutex) {
		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		for range ticker.C {
			if pingResponse, err := cc.redisClient.Ping().Result(); pingResponse != "PONG" || err != nil {
				cc.log.Critical("CANNOT PING REDIS. Resetting cluster and trying again.")
				mu.Lock()
				_, _ = cc.localClient.ClusterResetSoft().Result()
				mu.Unlock()
				pingResponse, err := cc.redisClient.Ping().Result()
				if err != nil {
					cc.log.Fatalf("could not ping redis. failing: %s: %v", pingResponse, err)
				}
			}
		}
	}(mu)

	cc.waitForRedis(ctx)

	// Keep node list up to date
	go cc.spawnPodWatcher(ctx, mu)

	joined := make(chan bool, 1)
	go func(joinedChannel chan bool) {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for range ticker.C {
			func() {
				err = cc.joinCluster(ctx)
				if err != nil {
					cc.log.Errorf("Couldn't join cluster: %v. Retrying in 30 seconds.", err)
					return
				}
				cc.log.Debug("Joined Cluster.")
				joined <- true
			}()
		}
	}(joined)
	<-joined

	// kick off a process that tries to find it's primary. if primary changes, adapt.
	cc.nodes = *cc.getClusterNodes(ctx, true)
	go func() {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for range ticker.C {
			cc.replicate(ctx, mu)
		}
	}()

	// ordzero has the special task of managing all cluster sharding
	if cc.isOrdZero() {
		go cc.rebalanceReplicas(ctx, mu)
	}

	cc.log.Info("===== Redis bootstrap complete =====")

	// Run until SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		cc.log.Infof("CAUGHT SIGNAL: %s", sig.String())
		mu.Lock()
		defer mu.Unlock()
		done <- true
	}()
	<-done
	cc.log.Info("Exiting")
}

func healtzHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	cc, ok := e.(*clusterConfigurator)
	if !ok {
		fmt.Printf("error getting cluster configurator: %v", reflect.TypeOf(e))
	}
	fmt.Fprintf(w, "Cluster Configurator: \n%+v\n", *cc)
	return nil
}

func readyzHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	cc, ok := e.(*clusterConfigurator)
	if !ok {
		panic("couldn't cast to *clusterConfigurator")
	}
	pingVal, err := cc.localClient.Ping().Result()
	if pingVal != "PONG" {
		w.WriteHeader(http.StatusInternalServerError)
		cc.log.Criticalf("Didn't get ping: %v: %v\n", pingVal, err)
	}
	fmt.Fprintf(w, "PING: %+v\n", pingVal)
	fmt.Fprintf(w, "Cluster Configurator: \n%+v\n", *cc)
	return nil
}

func isBetween(n, min, max int) bool {
	if n >= min && n <= max {
		return true
	}
	return false
}

func (targets *nodeReshardTargets) getExcessSlots(forNode *nodeReshardTarget, cmds *reshardCommands) {
	requiredSlots := forNode.nodeOffset
	for _, fromNode := range *targets {
		if fromNode.nodeOffset+forNode.nodeOffset >= 0 {
			// take everything forNode needs and add it to a cmd
			cmds.add(forNode.node.ID, fromNode.node.ID, forNode.nodeOffset*-1)
			return
		}
		// take everything the fromNode has to offer and move on.
		cmds.add(forNode.node.ID, fromNode.node.ID, fromNode.nodeOffset)
		requiredSlots = requiredSlots + fromNode.nodeOffset
		if requiredSlots == 0 {
			return
		}
		continue
	}
}

// blocks. only call from a goroutine
func (cc *clusterConfigurator) rebalanceReplicas(ctx context.Context, mu *sync.Mutex) {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for range ticker.C {
		// anon func so we can use defer and continue(or, return) at the same time.
		func() {
			mu.Lock()
			defer mu.Unlock()

			//get sum of all assigned slots
			cc.nodes = *cc.getClusterNodes(ctx, true)
			var allSlots clusterSlots
			allSlots, err := cc.redisClient.ClusterSlots().Result()
			if err != nil {
				cc.log.Criticalf("could not get ClusterSlots: %v", err)
				return
			}

			// if all slots are not assigned, just pick them up and assign them to Ord Zero
			var unassignedSlots []int
			if allSlots.sumOfSlots() != MAXSLOTS {
				for i := 0; i < MAXSLOTS; i++ {
					assigned := false
					for _, slot := range allSlots {
						if isBetween(i, slot.Start, slot.End) {
							assigned = true
						}
					}
					if !assigned {
						unassignedSlots = append(unassignedSlots, i)
					}
				}
				cc.log.Debugf("Taking slots over: %v", unassignedSlots)
				result, err := cc.redisClient.ClusterAddSlots(unassignedSlots...).Result()
				if err != nil {
					cc.log.Errorf("Could not add slots to cluster%v: result:%v. err: %v.\n", unassignedSlots, result, err)
				}
			}

			//calculate ideal distribution
			var primaryCount int
			for {
				primaryCount = cc.countOfPrimaryNodes()
				if primaryCount == 0 {
					cc.log.Criticalf("RESHARD: Count of primary nodes is 0... waiting and trying again to avoid Div/0 error.")
					time.Sleep(time.Second * 5)
					continue
				} else {
					break
				}
			}

			// 1) loop through all primary nodes and calculate the offset between ideal and actual assigned slots
			// 2) for each negative offset we create one or more reshard commands necessary to bring the offset to zero
			var primaryNodes clusterNodes
			// Only reshard primaries
			for _, node := range cc.nodes {
				if node.isPrimary {
					primaryNodes = append(primaryNodes, node)
				}
			}
			numberOfImbalancedNodes := MAXSLOTS % len(primaryNodes)

			var rebalanceTargets nodeReshardTargets
			for i, node := range primaryNodes {
				var reshardNode = &nodeReshardTarget{node: node}
				reshardNode.assignedSlotCount = node.getSlotsAssigned(allSlots).sumOfSlots()
				reshardNode.targetSlotCount = int(float64(MAXSLOTS / primaryCount))
				if i < numberOfImbalancedNodes {
					reshardNode.targetSlotCount = reshardNode.targetSlotCount + 1
				}
				reshardNode.nodeOffset = reshardNode.assignedSlotCount - reshardNode.targetSlotCount
				rebalanceTargets = append(rebalanceTargets, reshardNode)
			}
			cc.log.Debugf("DRYRUN: ImbalancedNodes: %d. MAXSLOTS: %d.", numberOfImbalancedNodes, MAXSLOTS)
			for _, target := range rebalanceTargets {
				cc.log.Debugf("DRYRUN TARGET: %+v", target)
			}
			var reshardCommands = &reshardCommands{}
			for _, target := range rebalanceTargets {
				if target.nodeOffset < 0 {
					rebalanceTargets.getExcessSlots(target, reshardCommands)
				}
			}
			cc.log.Debugf("DRYRUN: Reshard Commands: %+v", reshardCommands)
			for _, cmd := range *reshardCommands {
				cc.log.Debugf("DRYRUN CMD: %v", cmd)
				if cmd.count > 0 {
					cc.reshard(cmd.nodeFromID, cmd.nodeToID, strconv.Itoa(cmd.count))
				}
			}
		}()
	}

}

// blocks. only call from a goroutine
func (cc *clusterConfigurator) spawnPodWatcher(ctx context.Context, mu *sync.Mutex) {
	var resourceVersion string
	for {
		podChanges := cc.listenForPodChanges(ctx, cc.config.redisNamespace, cc.config.redisLabelSelector, resourceVersion)
		for event := range podChanges {
			p, ok := event.Object.(*v1.Pod)
			if !ok {
				log.Fatal("unexpected type")
			}
			switch event.Type {
			case "DELETED":
			case "BOOKMARK":
				resourceVersion = p.ResourceVersion
				cc.log.Debugf("Received bookmark: %s\n", resourceVersion)
			case "ADDED":
				fallthrough
			case "MODIFIED":
				if p.Status.Phase == v1.PodRunning {
					cc.log.Info("Pod Up")
					mu.Lock()
					cc.meetNewPeer(ctx, p.Status.PodIP)
					mu.Unlock()
				}
			default:
				cc.log.Infof("Unknown eventType: %v", event.Type)
			}
		}
		cc.log.Debugf("ClusterConfigurator Done. Restarting watch @%s", resourceVersion)
	}
}

func (nodes *clusterNodes) getOrdZero() *clusterNode {
	for _, node := range *nodes {
		nameSegments := strings.Split(node.podName, "-")
		if nameSegments[len(nameSegments)-1] == "0" {
			return node
		}
	}
	return nil
}
func (cc *clusterConfigurator) meetNewPeer(ctx context.Context, newNodeIP string) error {
	if cc.isOrdZero() {
		cc.log.Debugf("Heard about a new peer, saying hello: %s\n", newNodeIP)
		currentClient := *cc.redisClient
		_, err := currentClient.ClusterMeet(newNodeIP, cc.config.redisPort).Result()
		if err != nil {
			cc.log.Criticalf("Failed meet Ordinal Zero: %v", err)
			return err
		}
	}
	return nil
}

func (cc *clusterConfigurator) joinCluster(ctx context.Context) error {
	if !cc.isOrdZero() {
		err := cc.localClient.ClusterMeet(cc.getClusterNodes(ctx, false).getOrdZero().IPAddress, cc.config.redisPort).Err()
		if err != nil {
			cc.log.Criticalf("Failed to meet ord 0: %v", err)
			return err
		}
		return nil
	}
	return nil
}
func (cc *clusterConfigurator) myNode() *clusterNode {
	return cc.nodes.nodeByPodname(cc.config.podname)
}

func (cc *clusterConfigurator) replicate(ctx context.Context, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()
	cc.nodes = *cc.getClusterNodes(ctx, true)
	cc.log.Debugf("Checking replica status: MyNodeIs: %s", cc.config.podname)
	// TODO: only run if not already replica
	if cc.myNode().replicaOf != nil {
		if cc.myNode().replicaOf.ID != cc.myNode().currentPrimaryID {
			cc.log.Debugf("I'm a replica of %s, with ID of %s", cc.myNode().replicaOf.podName, cc.myNode().replicaOf.ID)
			replicateResult, err := cc.localClient.ClusterReplicate(cc.nodes.nodeByPodname(cc.config.podname).replicaOf.ID).Result()
			if err != nil {
				cc.log.Criticalf("Failed to replicate %s: %v", replicateResult, err)
				_, _ = cc.localClient.ClusterResetSoft().Result()
			} else {
				cc.nodes.nodeByPodname(cc.config.podname).replicated = true
			}
		}
	} else if cc.nodes.nodeByPodname(cc.config.podname).isPrimary && cc.myNode().currentReplicationState != "master" {
		resetResult, err := cc.localClient.ClusterResetSoft().Result()
		if err != nil {
			cc.log.Criticalf("could not reset local cluster state %s: %v", resetResult, err)
		}
		helloResult, err := cc.redisClient.ClusterMeet(cc.myNode().IPAddress, cc.config.redisPort).Result()
		if err != nil {
			cc.log.Criticalf("could not meet new node %s: %v", helloResult, err)
		}
	}
}

func (cc *clusterConfigurator) countOfPrimaryNodes() int {
	var count int
	for _, node := range cc.nodes {
		if node.isPrimary {
			count = count + 1
		}
	}
	return count
}
func (cmds *reshardCommands) add(toID string, fromID string, count int) {
	var maxOrder int
	for _, cmd := range *cmds {
		if cmd.order > maxOrder {
			maxOrder = cmd.order
		}
	}
	*cmds = append(*cmds, &reshardCommand{maxOrder + 1, toID, fromID, count})
}
func (cc *clusterConfigurator) reshard(nodeFromID string, nodeToID string, numberOfSlots string) {
	cmd := exec.Command("redis-cli",
		"--cluster", "reshard",
		"localhost:"+cc.config.redisPort,
		"--cluster-from", nodeFromID, "--cluster-to", nodeToID, "--cluster-slots", numberOfSlots, "--cluster-yes")

	logFile, err := os.OpenFile("/data/reshard.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		cc.log.Errorf("Could not create reshard.log: %v", err)
		return
	}
	errorFile, err := os.OpenFile("/data/reshard-error.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		cc.log.Errorf("Could not create reshard-error.log: %v", err)
		return
	}
	defer logFile.Close()
	defer errorFile.Close()

	logfileWriter := bufio.NewWriter(logFile)
	errorfileWriter := bufio.NewWriter(errorFile)

	defer logfileWriter.Flush()
	defer errorfileWriter.Flush()

	defer logfileWriter.WriteString(fmt.Sprintf("---------- %s ----------\n", time.Now().Format("2006-01-02T15:04:05-0700")))
	defer errorfileWriter.WriteString(fmt.Sprintf("---------- %s ----------\n", time.Now().Format("2006-01-02T15:04:05-0700")))

	cmd.Stdout = logfileWriter
	cmd.Stderr = errorfileWriter
	cc.log.Debugf("RESHARD: Running redis-cli command: %v", cmd.Args)
	err = cmd.Run()
	if err != nil {
		cc.log.Criticalf("Failed to run %s: %v", cmd.String(), err)
	}
}
func (clusterSlots clusterSlots) sumOfSlots() int {
	var sum int
	for _, slot := range clusterSlots {
		sum = sum + (slot.End - slot.Start) + 1
	}
	return sum
}
func (clusterNode *clusterNode) getSlotsAssigned(allSlots clusterSlots) clusterSlots {
	var retSlots clusterSlots
	for _, slot := range allSlots {
		for _, node := range slot.Nodes {
			//log.Printf("ASSIGNED: slotNode.ID: %s, clusterNode.ID: %s. equal: %t\n", node.ID, clusterNode.ID, node.ID == clusterNode.ID)
			if node.ID == clusterNode.ID {
				retSlots = append(retSlots, slot)
			}
		}
	}
	log.Printf("ASSIGNED: retSlots: %+v", retSlots)
	return retSlots
}

func (nodes clusterNodes) nodeByPodname(myPodname string) *clusterNode {
	for _, node := range nodes {
		if node.podName == myPodname {
			return node
		}
	}
	return nil
}
func (cc *clusterConfigurator) waitForRedis(ctx context.Context) {
	for {
		pong, err := cc.redisClient.Ping().Result()
		if err != nil || pong != "PONG" {
			cc.log.Infof("Error pinging redis: %s: %v", pong, err)
		} else {
			break
		}
		cc.log.Debugf("waiting for Redis to start. Got: %s", pong)
		time.Sleep(time.Second)
	}

	for {
		pod, err := cc.k8sClient.CoreV1().Pods(cc.config.redisNamespace).Get(ctx,
			cc.config.podname,
			metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Could not get pod info from Kubernetes API: %v.\n", err)
		}
		if pod.Status.Phase == v1.PodRunning && pod.Status.PodIP != "" {
			break
		}
	}
}
func (cc *clusterConfigurator) isOrdZero() bool {
	nameSegments := strings.Split(cc.config.podname, "-")
	return nameSegments[len(nameSegments)-1] == "0"
}

func (cc *clusterConfigurator) listenForPodChanges(ctx context.Context, namespace string, labelSelector string, bookmark string) <-chan watch.Event {
	var listOptions metav1.ListOptions
	if bookmark == "" {
		listOptions = metav1.ListOptions{LabelSelector: labelSelector}
	} else {
		listOptions = metav1.ListOptions{LabelSelector: labelSelector, ResourceVersion: bookmark, Watch: true}
	}
	watcher, err := cc.k8sClient.CoreV1().Pods(namespace).Watch(ctx, listOptions)
	if err != nil {
		cc.log.Criticalf("Could not establish watch: %v.\n", err)
		return nil
	}
	return watcher.ResultChan()
}

func (cc *clusterConfigurator) getClusterNodes(ctx context.Context, withRedisClient bool) *clusterNodes {
	listOptions := metav1.ListOptions{LabelSelector: cc.config.redisLabelSelector}
	statefulSet, err := cc.k8sClient.AppsV1().StatefulSets(cc.config.redisNamespace).Get(ctx, cc.config.redisStatefulsetName, metav1.GetOptions{})
	if err != nil {
		cc.log.Errorf("Could not get stateful set: %v\n", err)
		return nil
	}
	targetNumberOfNodes := *statefulSet.Spec.Replicas
	pods, err := cc.k8sClient.CoreV1().Pods(cc.config.redisNamespace).List(ctx, listOptions)
	if err != nil {
		cc.log.Criticalf("Could not get pods: %v.\n", err)
		return nil
	}
	var nodes clusterNodes
	for i := 0; i < len(pods.Items); i++ {
		if pods.Items[i].Status.Phase == v1.PodRunning {
			var newNode = &clusterNode{
				IPAddress: pods.Items[i].Status.PodIP,
				podName:   pods.Items[i].ObjectMeta.Name,
			}
			nodes = append(nodes, newNode)
		}
	}
	//do we need replicas?
	if targetNumberOfNodes/2 >= 3 && targetNumberOfNodes%2 == 0 {
		//make replicas
		sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].podName < nodes[j].podName })
		for i, node := range nodes {
			if i%2 == 0 {
				//make primary
				node.isPrimary = true
				if len(nodes) > 1+i {
					node.primaryFor = nodes[i+1]
				}
			} else {
				//make replica of previous primary
				node.replicaOf = nodes[i-1]
			}
		}
	}
	if withRedisClient {
		// user redis client to get ClusterNodes ID
		clusterNodes, err := cc.redisClient.ClusterNodes().Result()
		if err != nil {
			cc.log.Criticalf("Failed to get ClusterNodes with RedisClient: %v @%p", err, cc.redisClient)
		}
		scanner := bufio.NewScanner(strings.NewReader(clusterNodes))
		lineNumber := 0
		for scanner.Scan() {
			lineNumber = lineNumber + 1
			line := scanner.Text()
			// cc.log.Debugf("clusterNodes line %d: %s\n", lineNumber, line)
			nodesData := strings.Split(line, " ")
			for i := range nodesData {
				nodesData[i] = strings.TrimSpace(nodesData[i])
			}
			found := false
			for _, node := range nodes {
				if strings.HasPrefix(nodesData[1], node.IPAddress+":"+cc.config.redisPort) {
					found = true
					node.ID = nodesData[0]
					// unable to remove reference to slavery because these actual strings are returned from redis client
					for _, s := range strings.Split(nodesData[2], ",") {
						if s == "slave" {
							node.currentReplicationState = "slave"
							node.currentPrimaryID = nodesData[3]
							break
						} else if s == "master" {
							node.currentReplicationState = "master"
							break
						}
					}
					break
				}
			}
			if !found {
				_, err := cc.redisClient.ClusterForget(nodesData[0]).Result()
				cc.log.Debugf("Tried to forget %s: err:%v", nodesData[0], err)
			}
		}
	}
	return &nodes
}
func newKubernetesClient() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Could not create new Kubernetes Client. Error creating Kubernetes InClusterConfig: %s", err)
		return nil, err
	}
	// creates the clientset
	kubernetesClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Could create new Kubernetes Client. Error creating Kubernetes Client: %s", err)
		return nil, err
	}
	return kubernetesClient, nil
}
