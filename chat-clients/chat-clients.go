package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/logging"
	"github.com/aasmall/dicemagic/lib/envreader"
	"github.com/aasmall/dicemagic/lib/handler"
	log "github.com/aasmall/dicemagic/lib/logger"
	"github.com/go-redis/redis/v7"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

type externalClientType int

const (
	slackClientType externalClientType = iota
	diceServerClient
	httpClient
	webSocketClient
	redisClient
	datastoreClient
	kmsClient
)

type environment struct {
	config         *envConfig
	ShuttingDown   bool
	configReloader func() (*envConfig, error)
}
type externalClientsManager struct {
	slackClient      *SlackChatClient
	diceServerClient *grpc.ClientConn
	httpClient       *http.Client
	webSocketClient  *websocket.Dialer
	redisClient      *redis.ClusterClient
	datastoreClient  *datastore.Client
	kmsClient        *cloudkms.Service
	loggingClient    *log.Logger
}

func (ecm *externalClientsManager) getClient(ect externalClientType) interface{} {
	switch ect {
	case slackClientType:
		return ecm.slackClient
	case diceServerClient:
		return ecm.diceServerClient
	case httpClient:
		return ecm.httpClient
	case webSocketClient:
		return ecm.webSocketClient
	case redisClient:
		return ecm.redisClient
	case datastoreClient:
		return ecm.datastoreClient
	case kmsClient:
		return ecm.kmsClient
	default:
		return nil
	}
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
	mockDatastoreHost     string
	mockDatastorePort     string
	redisClusterHosts     []string
	debug                 bool
	local                 bool
}

// GetEnvironmentalConfigr reparses all data gathered from environment
// variables and Kubernetes
func getEnvironmentalConfig() (*envConfig, error) {
	// Gather Environment Variables
	configReader := new(envreader.EnvReader)
	config := &envConfig{
		projectID:             configReader.GetEnv("PROJECT_ID"),
		kmsKeyring:            configReader.GetEnv("KMS_KEYRING"),
		kmsSlackKey:           configReader.GetEnv("KMS_SLACK_KEY"),
		kmsSlackKeyLocation:   configReader.GetEnv("KMS_SLACK_KEY_LOCATION"),
		slackClientID:         configReader.GetEnv("SLACK_CLIENT_ID"),
		slackAppID:            configReader.GetEnv("SLACK_APP_ID"),
		slackOAuthDeniedURL:   configReader.GetEnv("SLACK_OAUTH_DENIED_URL"),
		logName:               configReader.GetEnv("LOG_NAME"),
		serverPort:            configReader.GetEnv("SERVER_PORT"),
		slackTokenURL:         configReader.GetEnv("SLACK_TOKEN_URL"),
		diceServerPort:        configReader.GetEnv("DICE_SERVER_PORT"),
		redisPort:             configReader.GetEnv("REDIS_PORT"),
		podName:               configReader.GetEnv("POD_NAME"),
		localRedirectURI:      configReader.GetEnvOpt("REDIRECT_URI"),
		slackProxyURL:         configReader.GetEnvOpt("SLACK_PROXY_URL"),
		mockKMSURL:            configReader.GetEnvOpt("MOCK_KMS_URL"),
		mockDatastoreHost:     configReader.GetEnvOpt("MOCK_DATASTORE_SERVICE_HOST"),
		mockDatastorePort:     configReader.GetEnvOpt("MOCK_DATASTORE_SERVICE_PORT"),
		debug:                 configReader.GetEnvBoolOpt("DEBUG"),
		local:                 configReader.GetEnvBoolOpt("LOCAL"),
		redisClusterHosts:     configReader.GetPodHosts("default", "k8s-app=redis"),
		encSlackSigningSecret: base64.StdEncoding.EncodeToString(configReader.GetFromFile("/etc/slack-secrets/slack-signing-secret")),
		encSlackClientSecret:  base64.StdEncoding.EncodeToString(configReader.GetFromFile("/etc/slack-secrets/slack-client-secret")),
	}
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
	env := &environment{configReloader: func() (*envConfig, error) { return getEnvironmentalConfig() }}
	err := env.ReloadConfig()
	if err != nil {
		log.Fatalf("ERROR OCCURED BEFORE LOGGING: %s", err)
	}
	ecm := &externalClientsManager{}
	ecm.loggingClient = log.New(
		env.config.projectID,
		log.WithDefaultSeverity(logging.Error),
		log.WithDebug(env.config.debug),
		log.WithLogName(env.config.logName),
		log.WithPrefix(env.config.podName+": "),
	)
	log := ecm.loggingClient
	log.Info("Logger up and running!")
	defer log.Info("Shutting down logger.")
	defer ecm.loggingClient.Close()

	//keep config up to date
	go func() {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for range ticker.C {
			err := env.ReloadConfig()
			if err != nil {
				log.Criticalf("Could not reload config: %v", err)
			}
		}
	}()

	// Default HTTP Client
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	// Default WebSocket Client
	if env.isLocal() {
		// override URL and HTTP client to force use of self-signed CA and mocks
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		certs, err := ioutil.ReadFile("/etc/mock-tls/tls.crt")
		if err != nil {
			log.Criticalf("Failed to append mock-server to RootCAs: %v", err)
		}
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			log.Debugf("No certs appended, using system certs only")
		}
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			RootCAs:            rootCAs,
		}
		netTransport.TLSClientConfig = tlsConfig

		// detect calls to slack API and redirect to mock slack-server
		netTransport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			log.Debugf("rewriting address: network: %s. address: %s", network, addr)
			if strings.HasPrefix(addr, "slack.com") {
				return tls.Dial(network, env.config.slackProxyURL, tlsConfig)
			}
			return tls.Dial(network, addr, tlsConfig)
		}
		ecm.webSocketClient = &websocket.Dialer{TLSClientConfig: tlsConfig}
	} else {
		ecm.webSocketClient = &websocket.Dialer{}
	}
	ecm.httpClient = &http.Client{Transport: netTransport}

	// Dice Server Client
	diceServerClient, err := grpc.Dial(env.config.diceServerPort, grpc.WithInsecure())
	if err != nil {
		log.Criticalf("did not connect to dice-server(%s): %v", env.config.diceServerPort, err)
		panic(err)
	}
	ecm.diceServerClient = diceServerClient

	// Redis Client
	log.Infof("Creating redis cluster client with URIs: %v\n", env.redisClusterAddresses())
	ecm.redisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    env.redisClusterAddresses(),
		Password: "",
	})

	// keep config up to date. Re-create redis client from k8s state if borked.
	go func() {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for range ticker.C {
			err = env.ReloadConfig()
			if err != nil {
				log.Criticalf("Failed to get new config: %v", err)
			}
			currentRedisClient := ecm.getClient(redisClient).(*redis.ClusterClient)
			if pingResponse, err := currentRedisClient.Ping().Result(); pingResponse != "PONG" || err != nil {
				log.Errorf("Lost connection to Redis Client, recreating: %s: %v", pingResponse, err)
				log.Debugf("Creating redis cluster client with URIs: %v\n", env.redisClusterAddresses())
				currentRedisClient.Close()
				ecm.redisClient = redis.NewClusterClient(&redis.ClusterOptions{
					Addrs:    env.redisClusterAddresses(),
					Password: "",
				})
			}
		}
	}()

	// Cloud Datastore Client
	var dsClient *datastore.Client
	if env.config.local {
		os.Setenv("DATASTORE_EMULATOR_HOST", env.config.mockDatastoreHost+":"+env.config.mockDatastorePort)
		os.Setenv("DATASTORE_EMULATOR_HOST_PATH", env.config.mockDatastoreHost+":"+env.config.mockDatastorePort+"/datastore")
		dsClient, err = datastore.NewClient(ctx,
			env.config.projectID, option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithInsecure()),
			option.WithGRPCDialOption(grpc.WithTimeout(time.Second*10)))
	} else {
		dsClient, err = datastore.NewClient(ctx, env.config.projectID)
	}
	if err != nil {
		log.Fatalf("Could not configure Datastore Client: %s", err)
	}
	ecm.datastoreClient = dsClient

	// Cloud KMS Client
	var kmsService *cloudkms.Service
	if env.config.local {
		kmsService, err = cloudkms.NewService(ctx,
			option.WithEndpoint(env.config.mockKMSURL),
			option.WithAPIKey("mockAPIKey"),
			option.WithHTTPClient(ecm.httpClient))
	} else {
		kmsService, err = cloudkms.NewService(ctx)
	}
	if err != nil {
		log.Fatalf("Error creating cloudKMS.service: %v", err)
	}
	ecm.kmsClient = kmsService

	// Call chat-clients init
	slackChatClient := env.NewSlackChatClient(ecm)

	// Define inbound Routes
	r := mux.NewRouter()
	r.Handle("/roll", handler.Handler{Env: ecm, H: RESTRollHandler})
	r.Handle("/slack/oauth", handler.Handler{Env: slackChatClient, H: SlackOAuthHandler})
	r.Handle("/slack/slash/roll", handler.Handler{Env: slackChatClient, H: SlackSlashRollHandler})
	r.Handle("/", handler.Handler{Env: env, H: rootHandler})

	// Define a server with timeouts
	srv := &http.Server{
		Addr:         env.config.serverPort,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux

	}
	srv.RegisterOnShutdown(slackChatClient.Init(ctx))

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Infof("ListenAndServe error: %+v", err)
		}
	}()

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/)
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
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
	log.Infof("shut down")
}

func rootHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
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
