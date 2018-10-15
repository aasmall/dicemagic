package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/logging"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"github.com/gorilla/mux"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
)

const (
	serverPort     = ":7070"
	diceServerPort = ":50051"
	logName        = "dicemagic-chat-clients-logs"
)

var (
	traceClient          *http.Client
	dsClient             *datastore.Client
	logger               *log.Logger
	projectID            string
	keyRing              string
	key                  string
	locationID           string
	slackClientID        string
	encSlackClientSecret string
	slackSuccessURL      string
	slackAccessDeniedURL string
	slackSigningSecret   string
)

func main() {
	ctx := context.Background()
	grpc.EnableTracing = true
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	// Gather Environment Variables
	projectID = os.Getenv("project-id")
	keyRing = os.Getenv("keyring")
	key = os.Getenv("slack-kms-key")
	locationID = os.Getenv("slack-kms-key-location-id")
	slackClientID = os.Getenv("slack-client-id")
	slackSigningSecret = os.Getenv("slack-signing-secret")
	encSlackClientSecret = os.Getenv("slack-client-secret")
	slackAccessDeniedURL = os.Getenv("slack-success-access-denied-redirect-url")

	// Stackdriver Trace exporter
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID: projectID,
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)
	traceClient = &http.Client{
		Transport: &ochttp.Transport{
			Propagation: &propagation.HTTPFormat{},
		},
	}
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	// Cloud Datastore Client
	dsClient, err = datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}

	// Creates a logging client.
	client, err := logging.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	logger = client.Logger(logName).StandardLogger(logging.Info)

	// Define inbound Routes
	r := mux.NewRouter()
	r.HandleFunc("/roll", QueryStringRollHandler)
	r.HandleFunc("/slack/oauth", SlackOAuthHandler)
	r.HandleFunc("/slack/slash/roll", SlackSlashRollHandler)
	r.HandleFunc("/", RootHandler)
	// http.Handle("/", r)

	h := &ochttp.Handler{Handler: r}

	// Define a server with timeouts
	srv := &http.Server{
		Addr:         serverPort,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      h, // Pass our instance of gorilla/mux and tracer in.

	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGQUIT)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	os.Exit(0)
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "200")
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
