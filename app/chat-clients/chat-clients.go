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
	"github.com/gorilla/mux"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
)

type envReader struct {
	missingKeys []string
	errors      bool
}

func (r *envReader) getEnv(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	r.errors = true
	r.missingKeys = append(r.missingKeys, key)
	return ""
}

type env struct {
	traceClient *http.Client
	dsClient    *datastore.Client
	config      *envConfig
	log         *logger.Logger
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
	traceProbability      float64
}

func main() {
	ctx := context.Background()

	// Gather Environment Variables
	configReader := new(envReader)
	traceProbability, ProbErr := strconv.ParseFloat(configReader.getEnv("TRACE_PROBABILITY"), 64)
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
		traceProbability:      traceProbability,
	}
	if configReader.errors {
		log.Fatalf("could not gather environment variables. Failed variables: %v", configReader.missingKeys)
	}
	if ProbErr != nil {
		log.Fatalf("could not convert TRACE_PROBABILITY environment variable to float64: %s", ProbErr)
	}
	env := &env{config: config}

	// Stackdriver Logger
	env.log = logger.NewLogger(ctx, env.config.projectID, env.config.logName)
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
	p := new(propagation.HTTPFormat)
	tc := &http.Client{
		Transport: &ochttp.Transport{
			// Use Google Cloud propagation format.
			Propagation: *p,
		},
	}
	env.traceClient = tc

	// Cloud Datastore Client
	dsClient, err := datastore.NewClient(ctx, env.config.projectID)
	if err != nil {
		log.Fatalf("could not configure Datastore Client: %s", err)
	}
	env.dsClient = dsClient

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

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGQUIT)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
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
