package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"github.com/gorilla/mux"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
)

const (
	inAddress     = ":7070"
	serverAddress = ":50051"
	logName       = "dicemagic-chat-clients"
	projectID     = "k8s-dice-magic"
	redirectURL   = "https://www.smallnet.org/"
)

var traceClient *http.Client

func main() {
	grpc.EnableTracing = true
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

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

	ctx, span := trace.StartSpan(context.Background(), "startup")
	defer span.End()

	// Get server addresses
	// serverHost, hostExists := os.LookupEnv("DICE_SERVER_SERVICE_SERVICE_HOST")
	// serverPort, portExists := os.LookupEnv("DICE_SERVER_SERVICE_SERVICE_PORT")
	// if hostExists && portExists {
	// 	serverAddress = fmt.Sprintf("%s:%s", serverHost, serverPort)
	// } else {
	// 	serverAddress = "localhost:50051"
	// }

	log.Printf("Initalized with serverAddress: %s", serverAddress)

	// Define inbound Routes
	r := mux.NewRouter()
	r.HandleFunc("/roll", QueryStringRollHandler)
	r.HandleFunc("/", RootHandler)
	http.Handle("/", r)

	h := &ochttp.Handler{Handler: r}

	// Define a server with timeouts
	srv := &http.Server{
		Addr:         inAddress,
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
