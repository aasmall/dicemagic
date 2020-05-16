package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aasmall/dicemagic/lib/dicelang"
	log "github.com/aasmall/dicemagic/lib/logger"
	"github.com/go-redis/redis"
	"google.golang.org/grpc"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/websocket"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

type SlackChatClient struct {
	SlackDatastoreClient
	log                 *log.Logger
	redisClient         redis.Cmdable
	traceClient         *http.Client
	diceClient          *grpc.ClientConn
	config              *envConfig
	slackConnectionPool map[int]*SlackConnection
	idGen               slack.IDGenerator
	mu                  sync.Mutex
	ShuttingDown        bool
	httpClient          *http.Client
	wssClient           *websocket.Dialer
}

type SlackDatastoreClient struct {
	*datastore.Client
	log *log.Logger
}
type SlackConnection struct {
	teamID      string
	botID       string
	oAuthDocKey *datastore.Key
	client      *slack.Client
	conn        *slack.RTM
	ID          int
}

// //SlackRollJSONResponse is the response format for slack commands
// type SlackRollJSONResponse struct {
// 	Text        string            `json:"text"`
// 	Attachments []SlackAttachment `json:"attachments"`
// }

// type SlackAttachment struct {
// 	Pretext    string       `json:"pretext"`
// 	Fallback   string       `json:"fallback"`
// 	Color      string       `json:"color"`
// 	AuthorName string       `json:"author_name"`
// 	Fields     []SlackField `json:"fields"`
// }
// type SlackField struct {
// 	Title string `json:"title"`
// 	Value string `json:"value"`
// 	Short bool   `json:"short"`
// }

func NewSlackChatClient(log *log.Logger, redisClient redis.Cmdable, datastoreClient *datastore.Client, traceClient *http.Client, diceClient *grpc.ClientConn, config *envConfig) *SlackChatClient {
	slackClient := &SlackChatClient{
		SlackDatastoreClient{datastoreClient, log},
		log,
		redisClient,
		traceClient,
		diceClient,
		config,
		make(map[int]*SlackConnection),
		slack.NewSafeID(1000),
		sync.Mutex{},
		false,
		http.DefaultClient,
		websocket.DefaultDialer,
	}
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	if config.local {
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
				return tls.Dial(network, config.slackProxyURL, tlsConfig)
			}
			return tls.Dial(network, addr, tlsConfig)
		}

		// override WSS dialer to use self-signed CA
		slackClient.wssClient = &websocket.Dialer{TLSClientConfig: tlsConfig}
	}
	slackClient.httpClient = &http.Client{Transport: netTransport}

	return slackClient
}

func returnErrorToSlack(text string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slack.Msg{Text: text})
}

func SlackAttachmentsFromRollResponse(rr *dicelang.RollResponse) []slack.Attachment {
	var sets []slack.Attachment
	// retSlackAttachment := slack.Attachment{
	// 	Fallback: totalsMapString(rr.DiceSet.TotalsByColor),
	// 	Color:    stringToColor(rr.DiceSet.ReString),
	// }
	dSets := dicelang.DiceSetsFromSlice(rr.DiceSets)
	retSlackAttachment := slack.Attachment{}
	retSlackAttachment.Fallback = totalsMapString(dSets.MergeDiceTotalMaps())
	retSlackAttachment.Color = stringToColor(dSets.String())

	var fields []slack.AttachmentField
	for _, ds := range rr.DiceSets {
		var faces []interface{}
		for _, d := range ds.Dice {
			faces = append(faces, facesSliceString(d.Faces))
		}
		field := slack.AttachmentField{
			Value: fmt.Sprintf(ds.ReString, faces...),
			Short: false,
		}
		field.Value = fmt.Sprintf("%s = *%s*", field.Value, strconv.FormatInt(ds.Total, 10))
		fields = append(fields, field)
	}
	if len(rr.DiceSets) > 1 {
		total, _ := dSets.GetTotal()
		fields = append(fields, slack.AttachmentField{
			Title: fmt.Sprintf("Total: %s", strconv.FormatInt(total, 10)),
			Short: false})
	}
	retSlackAttachment.Fields = fields
	sets = append(sets, retSlackAttachment)
	return sets
}

// ValidateSlackSignature checks the X-Slack-Signature slack appends
// to every request to ensure we're actually recieving them from slack.
func (sc *SlackChatClient) ValidateSlackSignature(r *http.Request) bool {
	log := sc.log.WithRequest(r)
	//read relevant headers
	slackSigString := r.Header.Get("X-Slack-Signature")
	remoteHMAC, _ := hex.DecodeString(strings.Split(slackSigString, "v0=")[1])
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")

	//read body and reset request
	body, err := ioutil.ReadAll(r.Body)
	log.Debug("body: " + string(body))
	if err != nil {
		log.Error("cannot validate slack signature. Cannot read body")
		return false
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	// check time skew
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Errorf("cannot validate slack signature. Cannot parse timestamp: %s", timestamp)
		return false
	}
	delta := time.Now().Sub(time.Unix(ts, 0))
	if delta.Minutes() > 5 {
		log.Errorf("cannot validate slack signature. Time skew > 5 minutes (%s)", delta.String())
		log.Debugf("timeskew: (%s)", delta.String())
		return false
	}

	decSigningSecret, err := sc.Decrypt(r.Context(), sc.config.kmsSlackKey, sc.config.encSlackSigningSecret)
	if err != nil {
		log.Errorf("cannot validate slack signature. can't decrypt signing secret: %s", err)
		return false
	}

	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	locahHMAC := CalculateHMAC(decSigningSecret, []byte(baseString))
	if hmac.Equal(remoteHMAC, locahHMAC) {
		return true
	}

	log.Debugf("baseString:  %s", baseString)
	log.Debugf("remoteHMAC: (%+v)\nlocahHMAC: (%+v)", hex.EncodeToString(remoteHMAC), hex.EncodeToString(locahHMAC))
	return false
}

func stringToColor(input string) string {
	bi := big.NewInt(0)
	h := md5.New()
	h.Write([]byte(input))
	hexb := h.Sum(nil)
	hexstr := hex.EncodeToString(hexb[:len(hexb)/2])
	bi.SetString(hexstr, 16)
	rand.Seed(bi.Int64())
	r := rand.Intn(0xff)
	g := rand.Intn(0xff)
	b := rand.Intn(0xff)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func (c *SlackChatClient) Init(ctx context.Context) func() {
	// advertise that I'm alive. Delete pods that aren't
	go c.ManageSlackConnections(ctx, time.Second*2)
	go c.SpawnPodCrier(time.Second * 5)
	go c.SpawnTeamsCrier(time.Second * 5)
	go c.SpawnReaper("pods", time.Second*10, time.Second*30)
	go c.SpawnReaper("teams", time.Second*10, time.Second*30)
	return func() { c.Cleanup() }
}

//Cleanup stops all long running go routines and disconnects all open websockets
func (c *SlackChatClient) Cleanup() {
	go func() {
		fmt.Println("cleaning up.")
		c.ShuttingDown = true
		c.DeletePod()
		c.RebalancePods(context.Background(), time.Second*5)
		for id := range c.slackConnectionPool {
			c.Disconnect(id)
			c.log.Debugf("killed connection for %d", id)
		}
		c.log.Debug("all cleaned up.")
	}()
}
