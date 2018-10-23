package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cloud.google.com/go/datastore"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/aasmall/dicemagic/app/handler"
	"github.com/aasmall/dicemagic/app/logger"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/nlopes/slack"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/grpc"
)

type env struct {
	traceClient        *http.Client
	datastoreClient    *datastore.Client
	config             *envConfig
	log                *logger.Logger
	diceServerClient   *grpc.ClientConn
	redisClient        *redis.Client
	redisClusterClient *redis.ClusterClient
	openRTMConnections map[string]*slack.RTM
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
	LocalRedirectUri      string
	debug                 bool
	traceProbability      float64
}

func main() {
	ctx := context.Background()

	// Gather Environment Variables
	configReader := new(envReader)
	config := &envConfig{
		projectID:             configReader.getEnv("PROJECT_ID"),
		kmsKeyring:            configReader.getEnv("KMS_KEYRING"),
		kmsSlackKey:           configReader.getEnv("KMS_SLACK_KEY"),
		kmsSlackKeyLocation:   configReader.getEnv("KMS_SLACK_KEY_LOCATION"),
		slackClientID:         configReader.getEnv("SLACK_CLIENT_ID"),
		slackAppID:            configReader.getEnv("SLACK_APP_ID"),
		encSlackSigningSecret: configReader.getEnv("SLACK_SIGNING_SECRET"),
		encSlackClientSecret:  configReader.getEnv("SLACK_CLIENT_SECRET"),
		slackOAuthDeniedURL:   configReader.getEnv("SLACK_OAUTH_DENIED_URL"),
		logName:               configReader.getEnv("LOG_NAME"),
		serverPort:            configReader.getEnv("SERVER_PORT"),
		slackTokenURL:         configReader.getEnv("SLACK_TOKEN_URL"),
		diceServerPort:        configReader.getEnv("DICE_SERVER_PORT"),
		redisPort:             configReader.getEnv("REDIS_PORT"),
		podName:               configReader.getEnv("POD_NAME"),
		LocalRedirectUri:      configReader.getEnv("REDIRECT_URI"),
		debug:                 configReader.getEnvBool("DEBUG"),
		traceProbability:      configReader.getEnvFloat("TRACE_PROBABILITY", 64),
	}
	if configReader.errors {
		log.Fatalf("could not gather environment variables. Failed variables: %v", configReader.missingKeys)
	}

	env := &env{config: config}
	env.openRTMConnections = make(map[string]*slack.RTM)

	// Stackdriver Logger
	env.log = logger.NewLogger(ctx, env.config.projectID, env.config.logName, env.config.debug)
	env.log.Info("Logger up and running!")
	defer log.Println("Shutting down logger.")
	defer env.log.Close()

	// Stackdriver Trace exporter
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

	// Cloud Datastore Client
	dsClient, err := datastore.NewClient(ctx, env.config.projectID)
	if err != nil {
		log.Fatalf("could not configure Datastore Client: %s", err)
	}
	env.datastoreClient = dsClient

	// Dice Server Client
	diceServerClient, err := grpc.Dial(env.config.diceServerPort,
		grpc.WithInsecure(),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		log.Fatalf("did not connect to dice-server(%s): %v", env.config.diceServerPort, err)
	}
	env.diceServerClient = diceServerClient

	// Redis Client
	env.redisClient = redis.NewClient(&redis.Options{
		Addr: env.config.redisPort,

		Password: "", // no password set
		DB:       0,  // use default DB
	})
	clusterIPs := []string{
		"redis-cluster-0.redis-cluster.default.svc.cluster.local:6379",
		"redis-cluster-1.redis-cluster.default.svc.cluster.local:6379",
		"redis-cluster-2.redis-cluster.default.svc.cluster.local:6379",
	}
	env.redisClusterClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    clusterIPs,
		Password: "",
	})

	// Define inbound Routes
	r := mux.NewRouter()
	r.Handle("/roll", handler.Handler{Env: env, H: QueryStringRollHandler})
	r.Handle("/slack/oauth", handler.Handler{Env: env, H: SlackOAuthHandler})
	r.Handle("/slack/slash/roll", handler.Handler{Env: env, H: SlackSlashRollHandler})
	r.Handle("/", handler.Handler{Env: env, H: RootHandler})

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

	// Call chat-clients init
	go ManageSlackConnections(ctx, env)
	go RebalancePods(ctx, env)

	// advertise that I'm alive. Delete pods that aren't
	go PingPods(env)
	go DeleteSleepingPods(env)

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		err := srv.ListenAndServe()
		log.Println("chat-clients up.")
		if err != nil {
			log.Fatal(err)
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
		// find and close all open slack RTM sessions
		for _, rtm := range env.openRTMConnections {
			rtm.Disconnect()
		}
		fmt.Println("shutting down...")
		srv.Shutdown(ctx)
	}()
	<-ctx.Done()
	log.Println("shut down")
	//os.Exit(0)
}

func RootHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	fmt.Fprint(w, "200")
	return nil
}

func TotalsMapString(m map[string]float64) string {
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
func FacesSliceString(faces []int64) string {
	var b [][]byte
	for _, f := range faces {
		b = append(b, []byte(strconv.FormatInt(f, 10)))
	}
	return string(bytes.Join(b, []byte(", ")))
}
