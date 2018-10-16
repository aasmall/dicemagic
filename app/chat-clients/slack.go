package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/aasmall/dicemagic/app/chat-clients/handler"

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

func SlackOAuthHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()
	ctx := r.Context()
	env, _ := e.(*env)
	log := env.logger.WithRequest(r)

	oauthError := r.FormValue("error")
	if oauthError == "access_denied" {
		http.Redirect(w, r, env.config.slackOAuthDeniedURL, http.StatusSeeOther)
		log.Infof("OAuth Access Denied, redirecting to: %s", env.config.slackOAuthDeniedURL)
		return nil
	}

	clientSecret, err := decrypt(ctx, env, env.config.encSlackClientSecret)
	if err != nil {
		log.Criticalf("error decrypting secret: %s", err)
		return err
	}

	form := url.Values{}
	form.Add("code", r.FormValue("code"))

	req, err := http.NewRequest("POST", env.config.slackTokenURL, strings.NewReader(form.Encode()))
	req.SetBasicAuth(env.config.slackClientID, clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.traceClient.Do(req)
	if err != nil {
		log.Criticalf("error getting oAuthToken: %s", err)
		return err
	}
	defer resp.Body.Close()

	var oauthResponse SlackInstallInstance
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		err := errors.New("error in oAuth Response")
		log.Criticalf("%s: %s", err, bodyString)
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(&oauthResponse)
	if err != nil {
		log.Criticalf("error decoding oAuth Response: %s", err)
		return err
	}

	EncBotAccessToken, err := encrypt(ctx, env, oauthResponse.Bot.BotAccessToken)
	if err != nil {
		log.Criticalf("error encrypting BotAccessToken: %s", err)
		return err
	}
	EncAccessToken, err := encrypt(ctx, env, oauthResponse.AccessToken)
	if err != nil {
		log.Criticalf("error encrypting AccessToken: %s", err)
		return err
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

	_, err = upsertSlackInstallInstance(ctx, env, newDoc)
	if err != nil {
		log.Errorf("error upserting SlackInstallInstance: %s", err)
		return err
	}

	slackSuccessURL := fmt.Sprintf("https://slack.com/app_redirect?app=%s", env.config.slackAppID)
	http.Redirect(w, r, slackSuccessURL, http.StatusSeeOther)

	return nil
}

func SlackSlashRollHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, _ := e.(*env)
	log := env.logger.WithRequest(r)

	//read body and reset request
	body, err := ioutil.ReadAll(r.Body)
	log.Debugf("body: %s", string(body))
	if err != nil {
		log.Critical("cannot validate slack signature. Cannot read body")
		return err
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	s, err := slack.SlashCommandParse(r)
	if err != nil {
		fmt.Fprintf(w, "could not parse slash command: %s", err)
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	if !ValidateSlackSignature(env, r) {
		return handler.StatusError{
			Code: http.StatusUnauthorized,
			Err:  errors.New("Invalid Slack Signature"),
		}
	}

	initd = dialDiceServer(env)
	rollerClient := pb.NewRollerClient(conn)
	timeOutCtx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	cmd := s.Text
	log.Debug(cmd)
	diceServerResponse, err := rollerClient.Roll(timeOutCtx, &pb.RollRequest{Cmd: cmd})
	if err != nil {
		fmt.Fprintf(w, "%+v: %+v", reflect.TypeOf(err), err)
		return nil
	}

	slackRollResponse := SlackRollJSONResponse{}

	slackRollResponse.Attachments = append(slackRollResponse.Attachments, SlackAttachmentFromRollResponse(diceServerResponse.DiceSet))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slackRollResponse)
	return nil
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
