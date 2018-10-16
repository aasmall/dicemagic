package logger

import (
	"fmt"
	"log"
	"net/http"

	"golang.org/x/net/context"

	"cloud.google.com/go/logging"
)

type Logger struct {
	stackDriverLogger *logging.Logger
	loggingClient     *logging.Client
	httpRequest       *logging.HTTPRequest
}

func NewLogger(ctx context.Context, projectID string, logName string) *Logger {
	logger := &Logger{}
	loggingClient, err := logging.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create logging client: %v", err)
	}
	logger.loggingClient = loggingClient
	logger.stackDriverLogger = loggingClient.Logger(logName)
	return logger
}

// WithRequest returns a shallow copy of logger with a request present
func (logger *Logger) WithRequest(r *http.Request) *Logger {
	if r == nil || logger == nil {
		panic("nil request")
	}
	logger2 := new(Logger)
	*logger2 = *logger
	logger2.httpRequest = &logging.HTTPRequest{Request: r}
	return logger2
}

func (logger *Logger) Info(message interface{}) {
	logger.log(logging.Entry{
		Payload:  message,
		Severity: logging.Info,
	})
}
func (logger *Logger) Debug(message interface{}) {
	logger.log(logging.Entry{
		Payload:  message,
		Severity: logging.Debug,
	})
}
func (logger *Logger) Error(message interface{}) {
	logger.log(logging.Entry{
		Payload:  message,
		Severity: logging.Error,
	})
}
func (logger *Logger) Critical(message interface{}) {
	logger.log(logging.Entry{
		Payload:  message,
		Severity: logging.Critical,
	})
}
func (logger *Logger) log(entry logging.Entry) {
	e := entry
	if logger.httpRequest != nil && entry.HTTPRequest == nil {
		e.HTTPRequest = logger.httpRequest
	}
	logger.stackDriverLogger.Log(e)
}
func (logger *Logger) Infof(format string, a ...interface{}) {
	logger.Info(fmt.Sprintf(format, a...))
}
func (logger *Logger) Debugf(format string, a ...interface{}) {
	logger.Debug(fmt.Sprintf(format, a...))
}
func (logger *Logger) Errorf(format string, a ...interface{}) {
	logger.Error(fmt.Sprintf(format, a...))
}
func (logger *Logger) Criticalf(format string, a ...interface{}) {
	logger.Critical(fmt.Sprintf(format, a...))
}

func (logger *Logger) Close() {
	logger.loggingClient.Close()
}
