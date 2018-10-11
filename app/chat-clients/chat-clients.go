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

	"cloud.google.com/go/logging"
	"github.com/gorilla/mux"
)

const (
	inAddress   = ":7070"
	logName     = "dicemagic-chat-clients"
	projectID   = "k8s-dice-magic"
	redirectURL = "https://www.smallnet.org/"
)

var serverAddress string

func main() {
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	// Create a logging client.
	client, err := logging.NewClient(context.Background(), projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	logger := client.Logger(logName).StandardLogger(logging.Info)
	// Get server addresses
	serverHost, hostExists := os.LookupEnv(os.Getenv("DICE_SERVER_SERVICE_SERVICE_HOST"))
	serverPort, portExists := os.LookupEnv(os.Getenv("DICE_SERVER_SERVICE_SERVICE_PORT"))
	if hostExists && portExists {
		serverAddress = fmt.Sprintf("%s:%s", serverHost, serverPort)
	} else {
		serverAddress = "localhost:50051"
	}
	logger.Printf("Initalized with serverAddress: %s", serverAddress)
	log.Printf("Initalized with serverAddress: %s", serverAddress)

	// Define inbound Routes
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("Redirecting to: %s", redirectURL)
		http.Redirect(w, r, redirectURL, 302)
	})
	r.HandleFunc("/roll", QueryStringRollHandler)
	http.Handle("/", r)

	// Define a server with timeouts
	srv := &http.Server{
		Addr:         inAddress,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux in.
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Fatalln(err)
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
