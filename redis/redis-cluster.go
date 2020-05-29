package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/logging"
	"github.com/aasmall/dicemagic/lib/envreader"
	log "github.com/aasmall/dicemagic/lib/logger"
	"github.com/go-redis/redis/v7"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//MAXSLOTS is the total number of slots a redis cluster has
const MAXSLOTS = 16383

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
	redisClient **redis.ClusterClient
	localClient *redis.Client
	log         *log.Logger
	config      *clusterConfiguratorConfig
	nodes       clusterNodes
}
type clusterNode struct {
	IPAddress  string
	ID         string
	master     bool
	slaveTo    *clusterNode
	masterTo   *clusterNode
	replicated bool
	PodName    string
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

type reshardCommands struct {
	commands []struct {
		order      int
		nodeToID   string
		nodeFromID string
		count      int
	}
}

func main() {
	ctx := context.Background()

	// Gather environment variables
	configReader := new(envreader.EnvReader)
	config := &clusterConfiguratorConfig{
		projectID:            configReader.GetEnv("PROJECT_ID"),
		debug:                configReader.GetEnvBoolOpt("DEBUG"),
		local:                configReader.GetEnvBoolOpt("LOCAL"),
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

	// Create new Kubernetes Client
	client, err := newKubernetesClient()
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}
	cc.k8sClient = client
	cc.waitForRedis(ctx)

	// if this is the first run, create a redis-client.
	var redisClientURIs []string
	for _, n := range *cc.getClusterNodes(ctx, false) {
		redisClientURIs = append(redisClientURIs, n.IPAddress+":"+cc.config.redisPort)
	}
	cc.log.Debug("Creating redis client on first init")
	redisClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    redisClientURIs,
		Password: "",
	})
	localClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // use default Addr
		Password: "",               // no password set
		DB:       0,                // use default DB
	})
	cc.localClient = localClient
	cc.redisClient = &redisClient
	cc.log.Debugf("Got redis client from %v", redisClientURIs)

	// keep config up to date. Re-create redis client from k8s state if borked.
	go func() {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for range ticker.C {
			currentClient := *cc.redisClient
			if pingResponse, err := currentClient.Ping().Result(); pingResponse != "PONG" || err != nil {
				cc.log.Errorf("Lost connection to Redis Client, recreating: %s: %v", pingResponse, err)
				cc.log.Debugf("Creating redis cluster client with URIs: %v\n", cc.getRedisClusterClientURIs(ctx))
				currentClient.Close()
				newClient := redis.NewClusterClient(&redis.ClusterOptions{
					Addrs:    cc.getRedisClusterClientURIs(ctx),
					Password: "",
				})
				cc.redisClient = &newClient
			}
		}
	}()

	// Keep node list up to date
	go func() {
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
						cc.meetNewPeer(ctx, p.Status.PodIP)
						//clusterConfigurator.redisClient.ReloadState()
					}
				default:
					cc.log.Infof("Unknown eventType: %v", event.Type)
				}
			}
			cc.log.Debugf("ClusterConfigurator Done. Restarting watch @%s", resourceVersion)
		}
	}()

	err = cc.joinCluster(ctx)
	if err != nil {
		time.Sleep(time.Second * 10)
		err = cc.joinCluster(ctx)
		if err != nil {
			log.Fatalf("Couldn't join cluster: %v", err)
		}
	}

	cc.createClusterIfMaster(ctx)

	// kick off a process that tries to find it's master. if master changes, adapt.
	cc.nodes = *cc.getClusterNodes(ctx, true)
	go func() {
		// if !clusterConfigurator.nodes.nodeByPodname(clusterConfigurator.config.podname).master {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for range ticker.C {
			cc.nodes = *cc.getClusterNodes(ctx, true)
			cc.replicateToMaster(ctx)
			// if clusterConfigurator.nodes.nodeByPodname(clusterConfigurator.config.podname).replicated == true {
			// 	break
			// }
		}
		// clusterConfigurator.log.Debug("Replicated to master successfully. Don't need to keep trying.")
		// }
	}()

	cc.log.Info("===== Redis bootstrap complete =====")

	// Run until SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		cc.log.Infof("CAUGHT SIGNAL: %s", sig.String())
		done <- true
	}()
	<-done
	cc.log.Info("Exiting")

	//get sum of all assigned slots
	// var totalSlots int
	// var allSlots clusterSlots
	// allSlots, err = clusterConfigurator.redisClient.ClusterSlots().Result()
	// totalSlots = allSlots.sumOfSlots()

	// //calculate ideal distribution
	// var masterCount int
	// for {
	// 	clusterConfigurator.getClusterNodes(ctx)
	// 	masterCount = clusterConfigurator.countOfMasterNodes()
	// 	if masterCount == 0 {
	// 		clusterConfigurator.log.Criticalf("RESHARD: Count of master nodes is 0.... waiting and trying again to avoid Div/0 error.")
	// 		time.Sleep(time.Second * 10)
	// 	} else {
	// 		break
	// 	}
	// }
	// targetSlotCount := int(math.Floor(float64(totalSlots / masterCount)))

	// //calculate number of imbalanced nodes
	// numberOfImbalancedNodes := targetSlotCount % len(clusterConfigurator.nodes)
	// clusterConfigurator.log.Debugf("RESHARD: totalSlots: %d. targetSlotCount: %d, numberofImbalancedNodes: %d", totalSlots, targetSlotCount, numberOfImbalancedNodes)

	// //for every known node, including ones with no slots assigned.
	// //sorted by node name
	// sort.SliceStable(clusterConfigurator.nodes, func(i, j int) bool {
	// 	return clusterConfigurator.nodes[i].PodName < clusterConfigurator.nodes[j].PodName
	// })
	// var reshardCommands = &reshardCommands{}
	// for i, node := range clusterConfigurator.nodes {
	// 	clusterConfigurator.log.Debugf("RESHARD: current node: %+v", node)
	// 	//Only reshard masters
	// 	if node.master {
	// 		//get the slots already assigned to this node
	// 		clusterConfigurator.log.Debugf("RESHARD: NodeIP(%s): %s:%s", node.PodName, node.IPAddress, node.ID)
	// 		currentNodeSlots := node.getSlotsAssigned(allSlots)
	// 		clusterConfigurator.log.Debugf("RESHARD: currentNodeSlots(%s): %+v", node.PodName, currentNodeSlots)
	// 		//count sum of slots assigned to current node and any that will be assigned during reshard
	// 		totalSlotsAssignedToNode := currentNodeSlots.sumOfSlots() + reshardCommands.countByToID(node.ID)
	// 		clusterConfigurator.log.Debugf("RESHARD: totalSlotsAssignedToNode(%s): %+v", node.PodName, totalSlotsAssignedToNode)
	// 		//if this node has more slots than it needs
	// 		if totalSlotsAssignedToNode > targetSlotCount {
	// 			//queue commsnd to move nodes
	// 			//from this node, to the next node in the loop
	// 			//if the node is imbalanced, add one slot
	// 			if i < numberOfImbalancedNodes {
	// 				reshardCommands.add(clusterConfigurator.nodes[i+1].ID, node.ID, (totalSlotsAssignedToNode-targetSlotCount)+1)
	// 			} else {
	// 				reshardCommands.add(clusterConfigurator.nodes[i+1].ID, node.ID, (totalSlotsAssignedToNode - targetSlotCount))
	// 			}
	// 		}
	// 	}
	// }

	// clusterConfigurator.log.Debugf("RESHARD: pending command: %+v", reshardCommands)
	// sort.SliceStable(reshardCommands.commands, func(i, j int) bool {
	// 	return reshardCommands.commands[i].order < reshardCommands.commands[j].order
	// })
	// if clusterConfigurator.isMaster() {
	// 	for _, reshardCommand := range reshardCommands.commands {
	// 		clusterConfigurator.reshard(reshardCommand.nodeFromID, reshardCommand.nodeToID, strconv.Itoa(reshardCommand.count))
	// 	}
	// }
}
func (cc *clusterConfigurator) getRedisClusterClientURIs(ctx context.Context) []string {
	var redisClientURIs []string
	for _, n := range *cc.getClusterNodes(ctx, false) {
		redisClientURIs = append(redisClientURIs, n.IPAddress+":"+cc.config.redisPort)
	}
	return redisClientURIs
}

func (nodes *clusterNodes) getOrdZero() *clusterNode {
	for _, node := range *nodes {
		nameSegments := strings.Split(node.PodName, "-")
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

func (cc *clusterConfigurator) replicateToMaster(ctx context.Context) {
	cc.log.Debugf("Checking slavery status: MyNodeIs: %s", cc.config.podname)
	if cc.nodes.nodeByPodname(cc.config.podname).slaveTo != nil {
		if cc.nodes.nodeByPodname(cc.config.podname).slaveTo.ID != "" {
			cc.log.Debugf("I'm a slave to %s, with ID of %s", cc.nodes.nodeByPodname(cc.config.podname).slaveTo.PodName, cc.nodes.nodeByPodname(cc.config.podname).slaveTo.ID)
			replicateResult, err := cc.localClient.ClusterReplicate(cc.nodes.nodeByPodname(cc.config.podname).slaveTo.ID).Result()
			if err != nil {
				cc.log.Criticalf("Failed to replicate %s: %v", replicateResult, err)
			} else {
				cc.nodes.nodeByPodname(cc.config.podname).replicated = true
			}
		}
	}
}
func (cc *clusterConfigurator) countOfMasterNodes() int {
	var count int
	for _, node := range cc.nodes {
		if node.master {
			count = count + 1
		}
	}
	return count
}
func (reshardCommands *reshardCommands) countByToID(ID string) int {
	var sum int
	for _, cmd := range reshardCommands.commands {
		if cmd.nodeToID == ID {
			sum = sum + cmd.count
		}
	}
	return sum
}
func (reshardCommands *reshardCommands) add(toID string, fromID string, count int) {
	var maxOrder int
	for _, reshardCommand := range reshardCommands.commands {
		if reshardCommand.order > maxOrder {
			maxOrder = reshardCommand.order
		}
	}
	reshardCommands.commands = append(reshardCommands.commands, struct {
		order      int
		nodeToID   string
		nodeFromID string
		count      int
	}{maxOrder, toID, fromID, count})
}
func (cc *clusterConfigurator) reshard(nodeFromID string, nodeToID string, numberOfSlots string) {
	cmd := exec.Command("redis-cli",
		"--cluster", "reshard",
		cc.nodes.nodeByPodname(cc.config.podname).IPAddress+":"+cc.config.redisPort,
		"--cluster-from", nodeFromID, "--cluster-to", nodeToID, "--cluster-slots", numberOfSlots, "--cluster-yes")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cc.log.Debugf("RESHARD: Running redis-cli command: %v", cmd.Args)
	err := cmd.Run()
	if err != nil {
		cc.log.Criticalf("Failed to run %s: %v", cmd.String(), err)
	}
	return
}
func (clusterNode *clusterNode) lowestSlot(allSlots clusterSlots) int {
	var min int
	slots := clusterNode.getSlotsAssigned(allSlots)
	for _, slot := range slots {
		if slot.Start < min {
			min = slot.Start
		}
	}
	return min
}
func (clusterSlots clusterSlots) sumOfSlots() int {
	var sum int
	for _, slot := range clusterSlots {
		sum = sum + slot.End - slot.Start
	}
	return sum
}
func (clusterNode *clusterNode) getSlotsAssigned(allSlots clusterSlots) clusterSlots {
	var retSlots clusterSlots
	for _, slot := range allSlots {
		for _, node := range slot.Nodes {
			log.Printf("ASSIGNED: slotNode.ID: %s, clusterNode.ID: %s. equal: %t\n", node.ID, clusterNode.ID, node.ID == clusterNode.ID)
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
		if node.PodName == myPodname {
			return node
		}
	}
	return nil
}
func (nodes clusterNodes) nodeByID(ID string) *clusterNode {
	for _, node := range nodes {
		if node.ID == ID {
			return node
		}
	}
	return nil
}
func (nodes clusterNodes) nodeByPodNameAndIP(PodName string, IP string) *clusterNode {
	for _, node := range nodes {
		if node.PodName == PodName && node.IPAddress == IP {
			return node
		}
	}
	return nil
}
func (cc *clusterConfigurator) waitForRedis(ctx context.Context) {
	for {
		cmd := exec.Command("redis-cli", "ping")
		var outb bytes.Buffer
		cmd.Stdout = &outb
		cmd.Stderr = os.Stderr
		err := cmd.Start()
		if err != nil {
			log.Fatalf("Error pinging redis: %v", err)
		}
		err = cmd.Wait()
		if err != nil {
			log.Fatalf("Error waiting for redis ping: %v", err)
		}
		if strings.Contains(outb.String(), "PONG") {
			break
		} else {
			cc.log.Debugf("waiting for Redis to start. Got: %s", outb.String())
		}
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
func (cc *clusterConfigurator) getNodeNames() []string {
	var nodeNames []string
	for _, node := range cc.nodes {
		nodeNames = append(nodeNames, node.PodName)
	}
	return nodeNames
}
func (cc *clusterConfigurator) deleteNodeByPodName(podName string) {
	for i, node := range cc.nodes {
		if node.PodName == podName {
			cc.log.Debug("REDIS: clusterConfigurator.redisClient.ClusterForget(node.ID).Result()")
			currentClient := *cc.redisClient
			_, err := currentClient.ClusterForget(node.ID).Result()
			if err != nil {
				cc.log.Criticalf("Failed to forget node: %v", err)
			}
			deleteNode(cc.nodes, i)
			if node.masterTo != nil {
				node.masterTo.replicated = false
			}
			break
		}
	}
}
func (cc *clusterConfigurator) getNodeByID(ID string) *clusterNode {
	for _, node := range cc.nodes {
		if node.ID == ID {
			return node
		}
	}
	return nil
}
func deleteNode(s clusterNodes, i int) clusterNodes {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}
func (cc *clusterConfigurator) isOrdZero() bool {
	nameSegments := strings.Split(cc.config.podname, "-")
	if nameSegments[len(nameSegments)-1] == "0" {
		return true
	}
	return false
}
func (cc *clusterConfigurator) createClusterIfMaster(ctx context.Context) {
	// If this is redis-0, take ownership of all slots
	if cc.isOrdZero() {
		cc.log.Debug("This node elected Ord Zero\n REDIS: clusterConfigurator.redisClient.ClusterAddSlotsRange(0, 16383).Result()")
		currentClient := *cc.redisClient
		addSlotsResult, err := currentClient.ClusterAddSlotsRange(0, 16383).Result()
		if err != nil {
			cc.log.Criticalf("Could not take ownership of all slots: %s: %v\n", addSlotsResult, err)
			currentClient := *cc.redisClient
			err := currentClient.FlushAll().Err()
			err = currentClient.ClusterResetSoft().Err()
			if err != nil {
				log.Fatalf("Failed to reset all nodes: %v\n", err)
			}
			cc.log.Debug("Reset all nodes, taking ownership again.")
			time.Sleep(time.Second * 10)
			addSlotsResult, err := currentClient.ClusterAddSlotsRange(0, 16383).Result()
			if err != nil {
				log.Fatalf("Could not take ownership of all slots: %s: %v\n", addSlotsResult, err)
			}
			cc.log.Debugf("took ownership of all slots after resetting nodes: %s", addSlotsResult)
		} else {
			cc.log.Debugf("took ownership of all slots: %s", addSlotsResult)
		}
	}
}

func getOrdZeroNode(nodes *clusterNodes) (*clusterNode, error) {
	for _, node := range *nodes {
		nameSegments := strings.Split(node.PodName, "-")
		if nameSegments[len(nameSegments)-1] == "0" {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no ordinal 0 pod found")
}
func seq(min, max int) []string {
	a := make([]string, max-min+1)
	for i := range a {
		a[i] = strconv.Itoa(min + i)
	}
	return a
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
				PodName:   pods.Items[i].ObjectMeta.Name,
			}
			nodes = append(nodes, newNode)
		}
	}
	//do we need replicas?
	if targetNumberOfNodes/2 >= 3 && targetNumberOfNodes%2 == 0 {
		//make slaves
		var b bytes.Buffer
		sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].PodName < nodes[j].PodName })
		b.WriteString("Assigning masters and slaves:\n")
		for i, node := range nodes {
			b.WriteString(fmt.Sprintf("PodName: %s: ", node.PodName))
			if i%2 == 0 {
				node.master = true
				if len(nodes) > 1+i {
					node.masterTo = nodes[i+1]
					b.WriteString("master\n")
				}
				//make master
			} else {
				node.slaveTo = nodes[i-1]
				b.WriteString(fmt.Sprintf("slave to %s\n", node.slaveTo.PodName))
				//make slave of previous master
			}
		}
		cc.log.Debug(b.String())
	}
	if withRedisClient {
		// user redis client to get ClusterNodes ID
		currentClient := *cc.redisClient
		clusterNodes, err := currentClient.ClusterNodes().Result()
		if err != nil {
			cc.log.Criticalf("Failed to get ClusterNodes with RedisClient: %v", err)
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
			for _, node := range nodes {
				if strings.HasPrefix(nodesData[1], node.IPAddress+":"+cc.config.redisPort) {
					node.ID = nodesData[0]
					break
				}
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
