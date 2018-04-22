package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"io/ioutil"
	"net/http"
	"os"
)

func slackOauthToken(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_KEY"), ctx)
}
func slackClientSecret(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_CLIENT_SECRET"), ctx)
}

func decrypt(ciphertext string, ctx context.Context) string {
	projectID := os.Getenv("PROJECT_ID")
	keyRing := os.Getenv("KMSKEYRING")
	key := os.Getenv("KMSKEY")
	locationID := "global"

	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%n", err))
	}

	kmsService, err := cloudkms.New(client)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%n", err))
	}
	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, locationID, keyRing, key)
	req := &cloudkms.DecryptRequest{
		Ciphertext: ciphertext,
	}
	resp, err := kmsService.Projects.Locations.KeyRings.CryptoKeys.Decrypt(parentName, req).Do()
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%n", err))
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
		log.Criticalf(ctx, fmt.Sprintf("%n", err))
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
		log.Criticalf(ctx, fmt.Sprintf("%n", err))
		return
	}
	if eventCallback.Token != slackClientSecret(ctx) {
		err := fmt.Errorf("Received Token does not match ClientSecret: %s", eventCallback.Token)
		log.Debugf(ctx, fmt.Sprintf("%s", err))
		return
	}
	switch eventCallback.InnerEvent.Type {
	case "app_mention":
		app_mentionHandler(InnerEvent(eventCallback.InnerEvent), w, r, ctx)
	default:
		log.Debugf(ctx, fmt.Sprintf("Unknown Callback Event: %n", eventCallback.InnerEvent.Type))
	}
}
func app_mentionHandler(innerEvent InnerEvent,
	w http.ResponseWriter,
	r *http.Request,
	ctx context.Context) {
	log.Debugf(ctx, fmt.Sprintf("%n", innerEvent))
}
func slackEventRouter(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	body, err := ioutil.ReadAll(r.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%n", err))
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
