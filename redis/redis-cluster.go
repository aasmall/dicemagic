package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	"github.com/aasmall/dicemagic/lib/envreader"
	log "github.com/aasmall/dicemagic/lib/logger"
	"github.com/go-redis/redis"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type clusterConfiguratorConfig struct {
	projectID          string
	debug              bool
	local              bool
	logname            string
	podname            string
	redisPort          string
	redisNamespace     string
	redisLabelSelector string
	localPodIP         string
}
type clusterConfigurator struct {
	k8sClient   *kubernetes.Clientset
	redisClient *redis.ClusterClient
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

func (clusterNodes clusterNodes) String() string {
	s := "["
	for i, node := range clusterNodes {
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
	configReader := new(envreader.EnvReader)
	ctx := context.Background()
	config := &clusterConfiguratorConfig{
		projectID:          configReader.GetEnv("PROJECT_ID"),
		debug:              configReader.GetEnvBoolOpt("DEBUG"),
		local:              configReader.GetEnvBoolOpt("LOCAL"),
		logname:            configReader.GetEnv("LOG_NAME"),
		podname:            configReader.GetEnv("POD_NAME"),
		redisPort:          configReader.GetEnv("REDIS_PORT"),
		redisNamespace:     configReader.GetEnv("REDIS_NAMESPACE"),
		redisLabelSelector: configReader.GetEnv("REDIS_LABEL_SELECTOR"),
		localPodIP:         configReader.GetEnv("LOCAL_POD_IP"),
	}
	if configReader.Errors {
		log.Fatalf("could not gather environment variables. Failed variables: %v", configReader.MissingKeys)
	}
	var clusterConfigurator = &clusterConfigurator{config: config}

	clusterConfigurator.log = log.New(
		clusterConfigurator.config.projectID,
		log.WithDefaultSeverity(logging.Error),
		log.WithDebug(clusterConfigurator.config.debug),
		log.WithLocal(clusterConfigurator.config.local),
		log.WithLogName(clusterConfigurator.config.logname),
		log.WithPrefix(clusterConfigurator.config.podname+": "),
	)
	client, err := newKubernetesClient()
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}
	clusterConfigurator.k8sClient = client

	// Create cluster on initial run
	clusterConfigurator.waitForRedis(ctx)
	clusterConfigurator.createCluster()
	clusterConfigurator.getClusterNodes(ctx)
	clusterConfigurator.meetPeers()
	clusterConfigurator.getClusterNodes(ctx)

	clusterConfigurator.log.Info("===== Redis bootstrap complete =====")

	// if I'm supposed to be a slave, replicate
	clusterConfigurator.findMaster(ctx)
	go func() {
		ticker := time.NewTicker(time.Second * 30)
		for range ticker.C {
			clusterConfigurator.findMaster(ctx)
		}
	}()

	var redisClientURIs []string
	for _, n := range clusterConfigurator.nodes {
		redisClientURIs = append(redisClientURIs, n.IPAddress+":"+clusterConfigurator.config.redisPort)
	}
	redisClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    redisClientURIs,
		Password: "",
	})
	clusterConfigurator.redisClient = redisClient
	clusterConfigurator.log.Debugf("Got new redis client from %v", redisClientURIs)
	//get sum of all assigned slots
	var totalSlots int
	var allSlots clusterSlots
	allSlots, err = redisClient.ClusterSlots().Result()
	totalSlots = allSlots.sumOfSlots()

	//calculate ideal distribution
	var masterCount int
	for {
		clusterConfigurator.getClusterNodes(ctx)
		masterCount = clusterConfigurator.countOfMasterNodes()
		if masterCount == 0 {
			clusterConfigurator.log.Criticalf("RESHARD: Count of master nodes is 0.... waiting and trying again to avoid Div/0 error.")
			time.Sleep(time.Second * 10)
		} else {
			break
		}
	}
	targetSlotCount := int(math.Floor(float64(totalSlots / masterCount)))

	//calculate number of imbalanced nodes
	numberOfImbalancedNodes := targetSlotCount % len(clusterConfigurator.nodes)
	clusterConfigurator.log.Debugf("RESHARD: totalSlots: %d. targetSlotCount: %d, numberofImbalancedNodes: %d", totalSlots, targetSlotCount, numberOfImbalancedNodes)

	//for every known node, including ones with no slots assigned.
	//sorted by node name
	sort.SliceStable(clusterConfigurator.nodes, func(i, j int) bool {
		return clusterConfigurator.nodes[i].PodName < clusterConfigurator.nodes[j].PodName
	})
	var reshardCommands = &reshardCommands{}
	for i, node := range clusterConfigurator.nodes {
		clusterConfigurator.log.Debugf("RESHARD: current node: %+v", node)
		//Only reshard masters
		if node.master {
			//get the slots already assigned to this node
			clusterConfigurator.log.Debugf("RESHARD: NodeIP(%s): %s:%s", node.PodName, node.IPAddress, node.ID)
			currentNodeSlots := node.getSlotsAssigned(allSlots)
			clusterConfigurator.log.Debugf("RESHARD: currentNodeSlots(%s): %+v", node.PodName, currentNodeSlots)
			//count sum of slots assigned to current node and any that will be assigned during reshard
			totalSlotsAssignedToNode := currentNodeSlots.sumOfSlots() + reshardCommands.countByToID(node.ID)
			clusterConfigurator.log.Debugf("RESHARD: totalSlotsAssignedToNode(%s): %+v", node.PodName, totalSlotsAssignedToNode)
			//if this node has more slots than it needs
			if totalSlotsAssignedToNode > targetSlotCount {
				//queue commsnd to move nodes
				//from this node, to the next node in the loop
				//if the node is imbalanced, add one slot
				if i < numberOfImbalancedNodes {
					reshardCommands.add(clusterConfigurator.nodes[i+1].ID, node.ID, (totalSlotsAssignedToNode-targetSlotCount)+1)
				} else {
					reshardCommands.add(clusterConfigurator.nodes[i+1].ID, node.ID, (totalSlotsAssignedToNode - targetSlotCount))
				}
			}
		}
	}

	clusterConfigurator.log.Debugf("RESHARD: pending command: %+v", reshardCommands)
	sort.SliceStable(reshardCommands.commands, func(i, j int) bool {
		return reshardCommands.commands[i].order < reshardCommands.commands[j].order
	})
	if clusterConfigurator.isMaster() {
		for _, reshardCommand := range reshardCommands.commands {
			clusterConfigurator.reshard(reshardCommand.nodeFromID, reshardCommand.nodeToID, strconv.Itoa(reshardCommand.count))
		}
	}
	for {
		podChanges := clusterConfigurator.listenForPodChanges(clusterConfigurator.config.redisNamespace, clusterConfigurator.config.redisLabelSelector)
		for event := range podChanges {
			fmt.Printf("Type: %v\n", event.Type)
			p, ok := event.Object.(*v1.Pod)
			if !ok {
				log.Fatal("unexpected type")
			}
			fmt.Println(p.Status.ContainerStatuses)
			fmt.Println(p.Status.Phase)
			switch event.Type {
			case "DELETED":
				fmt.Println("Deleted Pod: " + p.Name)
				clusterConfigurator.deleteNodeByPodName(p.Name)
			case "ADDED":
				fallthrough
			case "MODIFIED":
				if p.Status.Phase == v1.PodRunning {
					clusterConfigurator.log.Info("Pod Up")
					clusterConfigurator.getClusterNodes(ctx)
					if clusterConfigurator.isMaster() {
						clusterConfigurator.log.Info("I'm Master. Meeting all peers")
						clusterConfigurator.meetPeers()
					}
				}
			default:
				clusterConfigurator.log.Infof("Unknown eventType: %v", event.Type)
			}
		}
		clusterConfigurator.log.Debug("ClusterConfigurator Done. Restarting watch.")
	}
}

func (clusterConfigurator *clusterConfigurator) countOfMasterNodes() int {
	var count int
	for _, node := range clusterConfigurator.nodes {
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
func (clusterConfigurator *clusterConfigurator) reshard(nodeFromID string, nodeToID string, numberOfSlots string) {
	cmd := exec.Command("redis-cli",
		"--cluster", "reshard",
		clusterConfigurator.nodes.myNode(clusterConfigurator.config.podname).IPAddress+":"+clusterConfigurator.config.redisPort,
		"--cluster-from", nodeFromID, "--cluster-to", nodeToID, "--cluster-slots", numberOfSlots, "--cluster-yes")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	clusterConfigurator.log.Debugf("RESHARD: Running redis-cli command: %v", cmd.Args)
	err := cmd.Run()
	if err != nil {
		clusterConfigurator.log.Criticalf("Failed to run %s: %v", cmd.String(), err)
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
			log.Printf("ASSIGNED: slotNode.ID: %s, clusterNode.ID: %s. equal: %t\n", node.Id, clusterNode.ID, node.Id == clusterNode.ID)
			if node.Id == clusterNode.ID {
				retSlots = append(retSlots, slot)
			}
		}
	}
	log.Printf("ASSIGNED: retSlots: %+v", retSlots)
	return retSlots
}
func (clusterNodes clusterNodes) myNode(myPodname string) *clusterNode {
	for _, node := range clusterNodes {
		if node.PodName == myPodname {
			return node
		}
	}
	return nil
}
func (clusterNodes clusterNodes) nodeByID(ID string) *clusterNode {
	for _, node := range clusterNodes {
		if node.ID == ID {
			return node
		}
	}
	return nil
}
func (clusterConfigurator *clusterConfigurator) findMaster(ctx context.Context) {
	clusterConfigurator.getClusterNodes(ctx)
	clusterConfigurator.log.Debugf("Checking slavery status: MyNodeIs: %s", clusterConfigurator.config.podname)
	if clusterConfigurator.nodes.myNode(clusterConfigurator.config.podname).slaveTo != nil && clusterConfigurator.nodes.myNode(clusterConfigurator.config.podname).replicated == false {
		if clusterConfigurator.nodes.myNode(clusterConfigurator.config.podname).slaveTo.ID != "" {
			clusterConfigurator.log.Debugf("I'm a slave to %s, with ID of %s", clusterConfigurator.nodes.myNode(clusterConfigurator.config.podname).slaveTo.PodName, clusterConfigurator.nodes.myNode(clusterConfigurator.config.podname).slaveTo.ID)
			cmd := exec.Command("redis-cli", "cluster", "replicate", clusterConfigurator.nodes.myNode(clusterConfigurator.config.podname).slaveTo.ID)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				clusterConfigurator.log.Criticalf("Failed to run %s: %v", cmd.String(), err)
			}
			clusterConfigurator.nodes.myNode(clusterConfigurator.config.podname).replicated = true
		}
	}
}
func (clusterConfigurator *clusterConfigurator) waitForRedis(ctx context.Context) {
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
			clusterConfigurator.log.Debugf("waiting for Redis to start. Got: %s", outb.String())
		}
	}
	for {
		pod, err := clusterConfigurator.k8sClient.CoreV1().Pods(clusterConfigurator.config.redisNamespace).Get(ctx,
			clusterConfigurator.config.podname,
			metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Could not get pod info from Kubernetes API: %v.\n", err)
		}
		if pod.Status.Phase == v1.PodRunning && pod.Status.PodIP != "" {
			break
		}
	}
}
func (clusterConfigurator *clusterConfigurator) getNodeNames() []string {
	var nodeNames []string
	for _, node := range clusterConfigurator.nodes {
		nodeNames = append(nodeNames, node.PodName)
	}
	return nodeNames
}
func (clusterConfigurator *clusterConfigurator) deleteNodeByPodName(podName string) {
	for i, node := range clusterConfigurator.nodes {
		if node.PodName == podName {
			_, err := clusterConfigurator.redisClient.ClusterForget(node.ID).Result()
			if err != nil {
				clusterConfigurator.log.Criticalf("Failed to forget node: %v", err)
			}
			deleteNode(clusterConfigurator.nodes, i)
			if node.masterTo != nil {
				node.masterTo.replicated = false
			}
			break
		}
	}
}
func (clusterConfigurator *clusterConfigurator) getNodeByID(ID string) *clusterNode {
	for _, node := range clusterConfigurator.nodes {
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
func (clusterConfigurator *clusterConfigurator) isMaster() bool {
	nameSegments := strings.Split(clusterConfigurator.config.podname, "-")
	if nameSegments[len(nameSegments)-1] == "0" {
		return true
	}
	return false
}
func (clusterConfigurator *clusterConfigurator) createCluster() {
	// If this is redis-0, take ownership of all slots
	if clusterConfigurator.isMaster() {
		clusterConfigurator.log.Debug("This node elected leader")
		cmd := exec.Command("redis-cli", "cluster", "addslots")
		clusterConfigurator.log.Debugf("Running redis-cli command (slot numbers excluded): %v", cmd.Args)
		cmd.Args = append(cmd.Args, seq(0, 16383)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			clusterConfigurator.log.Criticalf("cmd.Run() failed with %s\n", err)
		}
	}
}
func (clusterConfigurator *clusterConfigurator) meetPeers() {
	for _, node := range clusterConfigurator.nodes {
		cmd := exec.Command("redis-cli", "cluster", "meet", node.IPAddress, clusterConfigurator.config.redisPort)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		clusterConfigurator.log.Debugf("Running redis-cli command: %v", cmd.Args)
		err := cmd.Run()
		if err != nil {
			clusterConfigurator.log.Criticalf("cmd.Run() failed with %s\n", err)
		}
	}
}
func seq(min, max int) []string {
	a := make([]string, max-min+1)
	for i := range a {
		a[i] = strconv.Itoa(min + i)
	}
	return a
}
func (clusterConfigurator *clusterConfigurator) listenForPodChanges(namespace string, labelSelector string) <-chan watch.Event {
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	watcher, err := clusterConfigurator.k8sClient.CoreV1().Pods(namespace).Watch(context.TODO(), listOptions)
	if err != nil {
		clusterConfigurator.log.Criticalf("Could not establish watch: %v.\n", err)
		return nil
	}
	return watcher.ResultChan()
}

func (clusterConfigurator *clusterConfigurator) getClusterNodes(ctx context.Context) {
	listOptions := metav1.ListOptions{LabelSelector: clusterConfigurator.config.redisLabelSelector}
	var nodes clusterNodes
	pods, err := clusterConfigurator.k8sClient.CoreV1().Pods(clusterConfigurator.config.redisNamespace).List(ctx, listOptions)
	if err != nil {
		clusterConfigurator.log.Criticalf("Could not get pods: %v.\n", err)
		return
	}
	for i := 0; i < len(pods.Items); i++ {
		if pods.Items[i].Status.Phase == v1.PodRunning {
			nodes = append(nodes, &clusterNode{
				IPAddress: pods.Items[i].Status.PodIP,
				PodName:   pods.Items[i].ObjectMeta.Name,
			})
		}
	}
	//do we need replicas?
	if len(nodes)/2 >= 3 && len(nodes)%2 == 0 {
		//make slaves
		for i, node := range nodes {
			if i%2 == 0 {
				node.master = true
				node.masterTo = nodes[i+1]
				//make master
			} else {
				node.slaveTo = nodes[i-1]
				//make slave of previous master
			}
		}
	}
	//get redis-cli cluster nodes
	cmd := exec.Command("redis-cli", "cluster", "nodes")
	var outb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		clusterConfigurator.log.Criticalf("Failed to run %s: %v", cmd.String(), err)
	}
	for {
		line, err := outb.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			clusterConfigurator.log.Criticalf("Failed to parse nodes: %v.\n", err)
			break
		}
		nodesData := strings.Split(line, " ")
		for i := range nodesData {
			nodesData[i] = strings.TrimSpace(nodesData[i])
		}
		//clusterConfigurator.log.Debugf("parsing nodesdata(len=%d)(ID=%s): %v", len(nodesData), nodesData[0], nodesData)
		if strings.Contains(nodesData[2], "myself") {
			nodes.myNode(clusterConfigurator.config.podname).ID = nodesData[0]
		} else {
			for _, node := range nodes {
				if strings.HasPrefix(nodesData[1], node.IPAddress+":"+clusterConfigurator.config.redisPort) {
					//clusterConfigurator.log.Debugf("found a matching prefix. %s ID: %s", node.PodName, nodesData[0])
					node.ID = nodesData[0]
					break
				} else {
					clusterConfigurator.log.Debugf("No match for nodedata: %v.", nodesData)
				}
			}
		}
	}
	clusterConfigurator.log.Debugf("got ClusterNodes: %+v", nodes)
	clusterConfigurator.nodes = nodes
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
