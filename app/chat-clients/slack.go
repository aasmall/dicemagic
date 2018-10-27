package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/go-redis/redis"

	"cloud.google.com/go/datastore"
	"github.com/aasmall/dicemagic/app/logger"
	pb "github.com/aasmall/dicemagic/app/proto"
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

//SlackRollJSONResponse is the response format for slack commands
type SlackRollJSONResponse struct {
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments"`
}

type SlackAttachment struct {
	Pretext    string       `json:"pretext"`
	Fallback   string       `json:"fallback"`
	Color      string       `json:"color"`
	AuthorName string       `json:"author_name"`
	Fields     []SlackField `json:"fields"`
}
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func NewSlackChatClient(log *log.Logger, redisClient redis.Cmdable, datastoreClient *datastore.Client, traceClient *http.Client, diceClient *grpc.ClientConn, config *envConfig) *SlackChatClient {
	return &SlackChatClient{
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
	}
}

func returnErrorToSlack(text string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SlackRollJSONResponse{Text: text})
}

func SlackAttachmentsFromRollResponse(rr *pb.RollResponse) []slack.Attachment {
	var sets []slack.Attachment
	retSlackAttachment := slack.Attachment{
		Fallback: totalsMapString(rr.DiceSet.TotalsByColor),
		Color:    stringToColor(rr.DiceSet.ReString),
	}
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
		fields = append(fields, slack.AttachmentField{
			Title: fmt.Sprintf("Total: %s", strconv.FormatInt(rr.DiceSet.Total, 10)),
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
	go c.SpawnTeamsCrier(time.Second * 10)
	go c.SpawnReaper("pods", time.Second*5, time.Second*15)
	go c.SpawnReaper("teams", time.Second*10, time.Second*15)
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
