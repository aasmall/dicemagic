package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/datastore"
	pb "github.com/aasmall/dicemagic/app/proto"
	"github.com/nlopes/slack"
)

type SlackInstallInstance struct {
	AccessToken string `json:"access_token"`
	Bot         struct {
		BotAccessToken string `json:"bot_access_token"`
		BotUserID      string `json:"bot_user_id"`
	} `json:"bot"`
	Ok       bool   `json:"ok"`
	Scope    string `json:"scope"`
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name"`
	UserID   string `json:"user_id"`
}

type SlackInstallInstanceDoc struct {
	Key         *datastore.Key `datastore:"__key__"`
	AccessToken string         `datastore:",noindex"`
	Bot         struct {
		EncBotAccessToken string `datastore:",noindex"`
		BotUserID         string `datastore:",noindex"`
	} `datastore:",noindex"`
	Scope    string `datastore:",noindex"`
	TeamID   string
	TeamName string `datastore:",noindex"`
	UserID   string
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

func SlackOAuthHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ctx := r.Context()
	slackTokenURL := "https://slack.com/api/oauth.access"

	oauthError := r.FormValue("error")
	if oauthError == "access_denied" {
		http.Redirect(w, r, slackAccessDeniedURL, http.StatusSeeOther)
		return
	}

	code := r.FormValue("code")

	clientSecret, err := decrypt(ctx, encSlackClientSecret)
	if err != nil {
		logger.Fatalf("error decrypting secret: %s", err)
	}

	form := url.Values{}
	form.Add("code", code)
	req, err := http.NewRequest("POST", slackTokenURL, strings.NewReader(form.Encode()))
	req.SetBasicAuth(slackClientID, clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := traceClient.Do(req)
	if err != nil {
		logger.Fatalf("error getting oAuthToken: %s", err)
	}
	defer resp.Body.Close()
	var oauthResponse SlackInstallInstance
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		logger.Printf("error in oAuth Response: %s", bodyString)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&oauthResponse)
	if err != nil {
		log.Fatalf("error decoding oAuth Response: %s", err)
	}

	EncBotAccessToken, err := encrypt(ctx, oauthResponse.Bot.BotAccessToken)
	if err != nil {
		log.Fatalf("error encrypting BotAccessToken: %s", err)
	}
	EncAccessToken, err := encrypt(ctx, oauthResponse.AccessToken)
	if err != nil {
		log.Fatalf("error encrypting AccessToken: %s", err)
	}

	newDoc := &SlackInstallInstanceDoc{
		AccessToken: EncAccessToken,
		Bot: struct {
			EncBotAccessToken string `datastore:",noindex"`
			BotUserID         string `datastore:",noindex"`
		}{
			EncBotAccessToken: EncBotAccessToken,
			BotUserID:         oauthResponse.Bot.BotUserID,
		},
		Scope:    oauthResponse.Scope,
		TeamID:   oauthResponse.TeamID,
		TeamName: oauthResponse.TeamName,
		UserID:   oauthResponse.UserID,
	}

	_, err = upsertSlackInstallInstance(ctx, newDoc)
	if err != nil {
		log.Printf("error upserting SlackInstallInstance: %s", err)
	}
	slackSuccessURL = fmt.Sprintf("https://slack.com/app_redirect?app=%s")

	http.Redirect(w, r, slackSuccessURL, http.StatusSeeOther)
}

func SlackSlashRollHandler(w http.ResponseWriter, r *http.Request) {

	//read body and reset request
	body, err := ioutil.ReadAll(r.Body)
	log.Println("body: " + string(body))
	if err != nil {
		log.Println("cannot validate slack signature. Cannot read body")
		return
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	s, err := slack.SlashCommandParse(r)
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	if err != nil {
		fmt.Fprintf(w, "could not parse slash command: %s", err)
	}
	dumpRequest(w, r)
	if !ValidateSlackSignature(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	initd = dialDiceServer()
	rollerClient := pb.NewRollerClient(conn)
	timeOutCtx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	cmd := "roll " + s.Text
	log.Println(cmd)
	diceServerResponse, err := rollerClient.Roll(timeOutCtx, &pb.RollRequest{Cmd: cmd})
	if err != nil {
		fmt.Fprintf(w, "could not roll: %v", err)
		return
	}

	slackRollResponse := SlackRollJSONResponse{}

	slackRollResponse.Attachments = append(slackRollResponse.Attachments, SlackAttachmentFromRollResponse(diceServerResponse.DiceSet))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slackRollResponse)
}

func SlackAttachmentFromRollResponse(ds *pb.DiceSet) SlackAttachment {
	retSlackAttachment := SlackAttachment{}
	var faces []interface{}
	for _, d := range ds.Dice {
		faces = append(faces, FacesSliceString(d.Faces))
	}

	for k, v := range ds.TotalsByColor {
		var fieldTitle string
		if k == "" && len(ds.TotalsByColor) == 1 {
			fieldTitle = ""
		} else if k == "" {
			fieldTitle = "Unspecified"
		} else {
			fieldTitle = k
		}
		field := SlackField{Title: fieldTitle, Value: strconv.FormatFloat(v, 'f', -1, 64), Short: true}
		retSlackAttachment.Fields = append(retSlackAttachment.Fields, field)
	}
	retSlackAttachment.Fallback = TotalsMapString(ds.TotalsByColor)
	retSlackAttachment.AuthorName = fmt.Sprintf(ds.ReString, faces...)
	retSlackAttachment.Color = stringToColor(ds.ReString)

	if len(ds.Dice) > 1 {
		field := SlackField{Title: "Total", Value: strconv.FormatInt(ds.Total, 10), Short: false}
		retSlackAttachment.Fields = append(retSlackAttachment.Fields, field)
	}

	return retSlackAttachment
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
