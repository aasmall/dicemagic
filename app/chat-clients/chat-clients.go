package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/logging"

	"cloud.google.com/go/datastore"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/aasmall/dicemagic/app/handler"
	"github.com/aasmall/dicemagic/app/logger"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/grpc"
)

type env struct {
	traceClient      *http.Client
	config           *envConfig
	log              *log.Logger
	diceServerClient *grpc.ClientConn
	ShuttingDown     bool
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
	redisClusterHosts     string
	debug                 bool
	local                 bool
	traceProbability      float64
}

func main() {
	log.Printf("hello.")
	ctx := context.Background()

	// Gather Environment Variables
	// TODO: run each in a go routine to spped up server init
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
		localRedirectURI:      configReader.getEnvOpt("REDIRECT_URI"),
		redisClusterHosts:     configReader.getEnvOpt("REDIS_CLUSTER_HOSTS"),
		debug:                 configReader.getEnvBoolOpt("DEBUG"),
		local:                 configReader.getEnvBoolOpt("LOCAL"),
		traceProbability:      configReader.getEnvFloat("TRACE_PROBABILITY"),
	}
	if configReader.errors {
		log.Fatalf("could not gather environment variables. Failed variables: %v", configReader.missingKeys)
	}

	env := &env{config: config}

	// Stackdriver Logger
	env.log = log.New(
		env.config.projectID,
		log.WithDefaultSeverity(logging.Error),
		log.WithDebug(env.config.debug),
		log.WithLocal(env.config.local),
		log.WithLogName(env.config.logName),
		log.WithPrefix(env.config.podName+": "),
	)
	env.log.Debug("Logger up and running!")
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

	// Dice Server Client
	diceServerClient, err := grpc.Dial(env.config.diceServerPort,
		grpc.WithInsecure(),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		log.Fatalf("did not connect to dice-server(%s): %v", env.config.diceServerPort, err)
	}
	env.diceServerClient = diceServerClient

	// Redis Client
	var redisClient redis.Cmdable
	if env.isLocal() {
		fmt.Printf("Creating redis client with port: %v\n", env.config.redisPort)
		redisClient = redis.NewClient(&redis.Options{
			Addr:     env.config.redisPort,
			Password: "", // no password set
			DB:       0,  // use default DB
		})
	} else {
		clusterURIs := strings.Split(env.config.redisClusterHosts, ";")
		for i, s := range clusterURIs {
			clusterURIs[i] = fmt.Sprintf("%s%s", strings.TrimSpace(s), env.config.redisPort)
		}
		env.log.Debugf("Creating redis cluster client with URIs: %v\n", clusterURIs)
		redisClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    clusterURIs,
			Password: "",
		})
	}

	// Cloud Datastore Client
	dsClient, err := datastore.NewClient(ctx, env.config.projectID)
	if err != nil {
		log.Fatalf("could not configure Datastore Client: %s", err)
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
	env := e.(*env)
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

func (env *env) isLocal() bool {
	return env.config.local
}
