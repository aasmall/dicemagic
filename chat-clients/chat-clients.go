package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/logging"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/aasmall/dicemagic/lib/envreader"
	"github.com/aasmall/dicemagic/lib/handler"
	log "github.com/aasmall/dicemagic/lib/logger"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

type environment struct {
	traceClient      *http.Client
	config           *envConfig
	log              *log.Logger
	diceServerClient *grpc.ClientConn
	ShuttingDown     bool
	configReloader   func() (*envConfig, error)
}

type envConfig struct {
	projectID             string
	kmsKeyring            string
	kmsSlackKey           string
	kmsSlackKeyLocation   string
	slackClientID         string
	encSlackSigningSecret string
	encSlackClientSecret  string
	slackOAuthDeniedURL   string
	logName               string
	serverPort            string
	diceServerPort        string
	slackTokenURL         string
	slackAppID            string
	redisPort             string
	podName               string
	localRedirectURI      string
	slackProxyURL         string
	mockKMSURL            string
	redisClusterHosts     []string
	debug                 bool
	local                 bool
	traceProbability      float64
}

func GetEnvironmentalConfig() (*envConfig, error) {
	// Gather Environment Variables
	configReader := new(envreader.EnvReader)
	config := &envConfig{
		projectID:           configReader.GetEnv("PROJECT_ID"),
		kmsKeyring:          configReader.GetEnv("KMS_KEYRING"),
		kmsSlackKey:         configReader.GetEnv("KMS_SLACK_KEY"),
		kmsSlackKeyLocation: configReader.GetEnv("KMS_SLACK_KEY_LOCATION"),
		slackClientID:       configReader.GetEnv("SLACK_CLIENT_ID"),
		slackAppID:          configReader.GetEnv("SLACK_APP_ID"),
		slackOAuthDeniedURL: configReader.GetEnv("SLACK_OAUTH_DENIED_URL"),
		logName:             configReader.GetEnv("LOG_NAME"),
		serverPort:          configReader.GetEnv("SERVER_PORT"),
		slackTokenURL:       configReader.GetEnv("SLACK_TOKEN_URL"),
		diceServerPort:      configReader.GetEnv("DICE_SERVER_PORT"),
		redisPort:           configReader.GetEnv("REDIS_PORT"),
		podName:             configReader.GetEnv("POD_NAME"),
		localRedirectURI:    configReader.GetEnvOpt("REDIRECT_URI"),
		slackProxyURL:       configReader.GetEnvOpt("SLACK_PROXY_URL"),
		mockKMSURL:          configReader.GetEnvOpt("MOCK_KMS_URL"),
		debug:               configReader.GetEnvBoolOpt("DEBUG"),
		local:               configReader.GetEnvBoolOpt("LOCAL"),
		traceProbability:    configReader.GetEnvFloat("TRACE_PROBABILITY"),
		redisClusterHosts:   configReader.GetPodHosts("default", "k8s-app=redis"),
	}
	config.encSlackSigningSecret = base64.StdEncoding.EncodeToString(configReader.GetFromFile("/etc/slack-secrets/slack-signing-secret"))
	config.encSlackClientSecret = base64.StdEncoding.EncodeToString(configReader.GetFromFile("/etc/slack-secrets/slack-client-secret"))
	// config.encSlackSigningSecret = strings.TrimSpace(config.encSlackSigningSecret)
	// config.encSlackClientSecret = strings.TrimSpace(config.encSlackClientSecret)
	if configReader.Errors {
		return nil, fmt.Errorf("Could not gather config. Failed variables: %v", configReader.MissingKeys)
	}
	return config, nil

}
func (env *environment) ReloadConfig() error {
	config, err := env.configReloader()
	if err != nil {
		return err
	}
	env.config = config
	return nil
}

func main() {
	log.Printf("hello.")
	ctx := context.Background()
	env := &environment{configReloader: func() (*envConfig, error) { return GetEnvironmentalConfig() }}
	err := env.ReloadConfig()
	if err != nil {
		log.Fatalf("ERROR OCCURED BEFORE LOGGING: %s", err)
	}
	// Stackdriver Logger
	env.log = log.New(
		env.config.projectID,
		log.WithDefaultSeverity(logging.Error),
		log.WithDebug(env.config.debug),
		log.WithLogName(env.config.logName),
		log.WithPrefix(env.config.podName+": "),
	)
	env.log.Info("Logger up and running!")
	defer log.Println("Shutting down logger.")
	defer env.log.Close()

	//keep config up to date
	go func() {
		ticker := time.NewTicker(time.Second * 60)
		defer ticker.Stop()
		for range ticker.C {
			err := env.ReloadConfig()
			if err != nil {
				env.log.Criticalf("Could not reload config: %v", err)
			}
		}
	}()

	// Stackdriver Trace exporter
	if !env.config.local {
		exporter, err := stackdriver.NewExporter(stackdriver.Options{
			ProjectID: env.config.projectID,
		})
		if err != nil {
			log.Fatalf("could not configure Stackdriver Exporter: %s", err)
		}
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.ProbabilitySampler(env.config.traceProbability)})
		trace.RegisterExporter(exporter)

		// Stackdriver Trace client
		grpc.EnableTracing = true
		env.traceClient = &http.Client{
			Transport: &ochttp.Transport{
				// Use Google Cloud propagation format.
				Propagation: *new(propagation.HTTPFormat),
			},
		}
	}
	// Dice Server Client
	diceServerClient, err := grpc.Dial(env.config.diceServerPort,
		grpc.WithInsecure(),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		log.Fatalf("did not connect to dice-server(%s): %v", env.config.diceServerPort, err)
	}
	env.diceServerClient = diceServerClient

	env.log.Infof("Creating redis cluster client with URIs: %v\n", env.redisClusterAddresses())
	redisClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    env.redisClusterAddresses(),
		Password: "",
	})
	go func() {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for range ticker.C {
			err = env.ReloadConfig()
			env.log.Debugf("Creating redis cluster client with URIs: %v\n", env.redisClusterAddresses())
			*redisClient = *redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:    env.redisClusterAddresses(),
				Password: "",
			})
		}
	}()
	// Keep ClusterIPs up to date if IPs change
	// go func(env *environment, client *redis.ClusterClient) {
	// 	ticker := time.NewTicker(time.Second * 30)
	// 	defer ticker.Stop()
	// 	for range ticker.C {
	// 		err = env.ReloadConfig()
	// 		env.log.Debugf("reloading redisClientState with URIs: %v\n", env.redisClusterAddresses())
	// 		err = redisClient.ReloadState()
	// 		if err != nil {
	// 			env.log.Criticalf("Error updating Redis ClusterClientConfig: %v", err)
	// 		}
	// 	}
	// }(env, redisClient)

	// Cloud Datastore Client
	var dsClient *datastore.Client
	if env.config.local {
		dsClient, err = datastore.NewClient(ctx, env.config.projectID, option.WithoutAuthentication(), option.WithGRPCDialOption(grpc.WithInsecure()))
	} else {
		dsClient, err = datastore.NewClient(ctx, env.config.projectID)
	}
	if err != nil {
		log.Fatalf("Could not configure Datastore Client: %s", err)
	}

	// Call chat-clients init
	slackChatClient := NewSlackChatClient(env.log, redisClient, dsClient, env.traceClient, env.diceServerClient, env.config)

	// Define inbound Routes
	r := mux.NewRouter()
	r.Handle("/roll", handler.Handler{Env: env, H: RESTRollHandler})
	r.Handle("/slack/oauth", handler.Handler{Env: slackChatClient, H: SlackOAuthHandler})
	r.Handle("/slack/slash/roll", handler.Handler{Env: slackChatClient, H: SlackSlashRollHandler})
	r.Handle("/", handler.Handler{Env: env, H: rootHandler})

	// Add OpenCensus HTTP Handler Wrapper
	openCensusWrapper := &ochttp.Handler{Handler: r}

	// Define a server with timeouts
	srv := &http.Server{
		Addr:         env.config.serverPort,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      openCensusWrapper, // Pass our instance of gorilla/mux and tracer in.

	}
	srv.RegisterOnShutdown(slackChatClient.Init(ctx))

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Printf("ListenAndServe error: %+v", err)
		}
	}()

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/)
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	go func() {
		srv.Shutdown(ctx)
	}()
	<-ctx.Done()
	log.Println("shut down")
	//os.Exit(0)
}

func rootHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env := e.(*environment)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.NeverSample()})
	defer trace.ApplyConfig(trace.Config{DefaultSampler: trace.ProbabilitySampler(env.config.traceProbability)})
	fmt.Fprint(w, "200")
	return nil
}

func totalsMapString(m map[string]float64) string {
	var b [][]byte
	if len(m) == 1 && m[""] != 0 {
		return strconv.FormatFloat(m[""], 'f', 1, 64)
	}
	for k, v := range m {
		if k == "" {
			b = append(b, []byte("Unspecified"))
		} else {
			b = append(b, []byte(k))
		}
		b = append(b, []byte(": "))
		b = append(b, []byte(strconv.FormatFloat(v, 'f', 1, 64)))
	}
	return string(bytes.Join(b, []byte(", ")))
}
func facesSliceString(faces []int64) string {
	var b [][]byte
	for _, f := range faces {
		b = append(b, []byte(strconv.FormatInt(f, 10)))
	}
	return string(bytes.Join(b, []byte(", ")))
}

func (env *environment) isLocal() bool {
	return env.config.local
}
func (env *environment) redisClusterAddresses() []string {
	clusterURIs := make([]string, len(env.config.redisClusterHosts))
	copy(clusterURIs, env.config.redisClusterHosts)
	for i, s := range clusterURIs {
		clusterURIs[i] = fmt.Sprintf("%s:%s", strings.TrimSpace(s), env.config.redisPort)
	}
	return clusterURIs
}
func (env *environment) redisClusterSlots() ([]redis.ClusterSlot, error) {
	addresses := env.redisClusterAddresses()
	sort.Strings(addresses)
	//client side cluster
	slots := splitSlots(len(addresses))

	for i := 0; i < len(slots); i++ {
		slots[i].Nodes = []redis.ClusterNode{{Addr: addresses[i]}}
	}
	env.log.Debugf("redisClusterSet = %+v", slots)
	return slots, nil
}
func splitSlots(numberOfHosts int) []redis.ClusterSlot {
	maxSlots := 16383
	var slots []redis.ClusterSlot
	nextSlot := 0
	if numberOfHosts == 0 {
		log.Printf("redis hosts not up yet, cannot assign slots.")
		return nil
	}
	if maxSlots < numberOfHosts {
		// More hosts than slots, can't be divided
		return []redis.ClusterSlot{{Start: 0}, {End: 0}}
	} else if (maxSlots % numberOfHosts) == 0 {
		// slots divide evenly into hosts
		for i := 0; i < numberOfHosts; i++ {
			slotsEach := maxSlots / numberOfHosts
			slots = append(slots, redis.ClusterSlot{Start: nextSlot, End: (i + 1) * slotsEach})
			nextSlot = ((i + 1) * slotsEach) + 1
		}
	} else {
		// slots do not divide evenly into hosts
		unevenCount := numberOfHosts - (maxSlots % numberOfHosts)
		evenAmount := maxSlots / numberOfHosts
		for i := 0; i < numberOfHosts; i++ {
			if i >= unevenCount {
				slots = append(slots, redis.ClusterSlot{Start: nextSlot, End: (i + 1) * (evenAmount + 1)})
				nextSlot = ((i + 1) * (evenAmount + 1)) + 1
			} else {
				slots = append(slots, redis.ClusterSlot{Start: nextSlot, End: (i + 1) * evenAmount})
				nextSlot = ((i + 1) * evenAmount) + 1
			}
		}
	}
	return slots
}
