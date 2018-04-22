package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
)

var naturalLanguageRegexp = regexp.MustCompile(`(?i)^.*roll\s+(?P<expression>\d+d\d+.+)$`)

func slackOauthToken(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_KEY"), ctx)
}
func slackClientSecret(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_CLIENT_SECRET"), ctx)
}
func slackBotAccessToken(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_BOT_USER_ACCES_TOKEN"), ctx)
}

func decrypt(ciphertext string, ctx context.Context) string {
	projectID := os.Getenv("PROJECT_ID")
	keyRing := os.Getenv("KMSKEYRING")
	key := os.Getenv("KMSKEY")
	locationID := "global"

	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}

	kmsService, err := cloudkms.New(client)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, locationID, keyRing, key)
	req := &cloudkms.DecryptRequest{
		Ciphertext: ciphertext,
	}
	resp, err := kmsService.Projects.Locations.KeyRings.CryptoKeys.Decrypt(parentName, req).Do()
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	decodedString, _ := base64.StdEncoding.DecodeString(resp.Plaintext)
	return string(decodedString)
}

func slackRoll(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	content := r.FormValue("text")
	rollResult := evaluate(parse(content))

	fmt.Fprintf(w, "You rolled %d\n", rollResult)
}

func url_verificationHandler(body []byte, w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var challengeRequest ChallengeRequest
	err := json.Unmarshal(body, &challengeRequest)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	if challengeRequest.Token != slackClientSecret(ctx) {
		err := fmt.Errorf("Received Token does not match ClientSecret: %s", challengeRequest.Token)
		log.Debugf(ctx, fmt.Sprintf("%s", err))
		return
	}
	fmt.Fprintln(w, challengeRequest.Challenge)

}
func event_callbackHandler(body []byte, w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var eventCallback EventCallback
	err := json.Unmarshal(body, &eventCallback)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	if eventCallback.Token != slackClientSecret(ctx) {
		err := fmt.Errorf("Received Token does not match ClientSecret: %+v", eventCallback.Token)
		log.Debugf(ctx, fmt.Sprintf("%s", err))
		return
	}
	switch eventCallback.InnerEvent.Type {
	case "app_mention":
		app_mentionHandler(InnerEvent(eventCallback.InnerEvent), w, r, ctx)
	default:
		log.Debugf(ctx, fmt.Sprintf("Unknown Callback Event: %+v", eventCallback.InnerEvent.Type))
	}
}
func app_mentionHandler(innerEvent InnerEvent,
	w http.ResponseWriter,
	r *http.Request,
	ctx context.Context) {
	resultOfDice := parseMentionAndRoll(innerEvent.Text)
	text := fmt.Sprintf("<@%s> rolled %d", innerEvent.User, resultOfDice)
	log.Debugf(ctx, fmt.Sprintf("%+v", text))
	postTextToChannel(ctx, innerEvent.Channel, text)
	log.Debugf(ctx, fmt.Sprintf("%+v", innerEvent))

}
func parseMentionAndRoll(text string) int64 {
	match := naturalLanguageRegexp.FindStringSubmatch(text)
	paramsMap := make(map[string]string)
	for i, name := range naturalLanguageRegexp.SubexpNames() {
		if i > 0 && i <= len(match) {
			paramsMap[name] = match[i]
		}
	}
	roll := evaluate(parse(paramsMap["expression"]))
	return roll
}
func postTextToChannel(ctx context.Context, channel string, text string) {
	methodUrl := "https://slack.com/api/chat.postMessage"

	client := urlfetch.Client(ctx)

	message := new(Message)
	message.Text = text
	message.Channel = channel
	b, err := json.Marshal(message)
	req, err := http.NewRequest("POST", methodUrl, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", slackBotAccessToken(ctx)))
	resp, err := client.Do(req)

	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	defer resp.Body.Close()
}
func slackEventRouter(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	body, err := ioutil.ReadAll(r.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	defer r.Body.Close()
	eventType := result["type"]
	switch eventType {
	case "url_verification":
		url_verificationHandler(body, w, r, ctx)
	case "event_callback":
		event_callbackHandler(body, w, r, ctx)
	default:
	}
}
