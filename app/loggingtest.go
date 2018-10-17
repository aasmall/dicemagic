package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/aasmall/dicemagic/app/handler"

	"log"

	"cloud.google.com/go/logging"
	"golang.org/x/net/context"
)

type chatClientsLogger struct {
	stackDriverLogger *logging.Logger
	loggingClient     *logging.Client
	httpRequest       *logging.HTTPRequest
}

func NewLogger(ctx context.Context, projectID string, logName string) *chatClientsLogger {
	logger := &chatClientsLogger{}
	logger.New(ctx, projectID, logName)
	return logger
}

func (logger *chatClientsLogger) New(ctx context.Context, projectID string, logName string) {
	loggingClient, err := logging.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create logging client: %v", err)
	}
	logger.loggingClient = loggingClient
	logger.stackDriverLogger = loggingClient.Logger(logName)
}

// WithRequest returns a shallow copy of logger with a request present
func (logger *chatClientsLogger) WithRequest(r *http.Request) *chatClientsLogger {
	if r == nil || logger == nil {
		panic("nil request")
	}
	logger2 := new(chatClientsLogger)
	*logger2 = *logger
	logger2.httpRequest = &logging.HTTPRequest{Request: r}
	return logger2
}

func (logger *chatClientsLogger) Info(message interface{}) {
	logger.Log(logging.Entry{
		Payload:  message,
		Severity: logging.Info,
	})
}
func (logger *chatClientsLogger) Debug(message interface{}) {
	logger.Log(logging.Entry{
		Payload:  message,
		Severity: logging.Debug,
	})
}
func (logger *chatClientsLogger) Error(message interface{}) {
	logger.Log(logging.Entry{
		Payload:  message,
		Severity: logging.Error,
	})
}
func (logger *chatClientsLogger) Critical(message interface{}) {
	logger.Log(logging.Entry{
		Payload:  message,
		Severity: logging.Critical,
	})
}
func (logger *chatClientsLogger) Log(entry logging.Entry) {
	e := entry
	if logger.httpRequest != nil && entry.HTTPRequest == nil {
		e.HTTPRequest = logger.httpRequest
	}
	logger.stackDriverLogger.Log(e)
}
func (logger *chatClientsLogger) Infof(format string, a ...interface{}) {
	logger.Info(fmt.Sprintf(format, a...))
}
func (logger *chatClientsLogger) Debugf(format string, a ...interface{}) {
	logger.Debug(fmt.Sprintf(format, a...))
}
func (logger *chatClientsLogger) Errorf(format string, a ...interface{}) {
	logger.Error(fmt.Sprintf(format, a...))
}
func (logger *chatClientsLogger) Criticalf(format string, a ...interface{}) {
	logger.Critical(fmt.Sprintf(format, a...))
}

func (logger *chatClientsLogger) Close() {
	logger.loggingClient.Close()
}

type env struct {
	logger *chatClientsLogger
}

func main() {

	// Stackdriver Logger
	logger := NewLogger(context.Background(), "k8s-dice-magic", "dicemagic-chat-clients-logs")
	defer logger.Close()

	env := &env{}
	env.logger = logger

	http.Handle("/", handler.Handler{Env: env, H: RootHandler})
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
func RootHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, ok := e.(*env)
	if !ok {
		return handler.StatusError{500, errors.New("UNEXPECTED: could not convert interface to *env")}
	}
	// Sets your Google Cloud Platform project ID.
	log := env.logger.WithRequest(r)

	for index := 0; index < 5; index++ {
		log.Error("Log with r")
	}
	return nil
}
