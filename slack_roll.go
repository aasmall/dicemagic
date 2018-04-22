package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	//"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/appengine"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func slackOauthToken(r *http.Request) string {
	return decrypt(os.Getenv("SLACK_KEY"), r)
}
func slackClientSecret(r *http.Request) string {
	return decrypt(os.Getenv("SLACK_CLIENT_SECRET"), r)
}

func decrypt(ciphertext string, r *http.Request) string {
	projectID := os.Getenv("PROJECT_ID")
	keyRing := os.Getenv("KMSKEYRING")
	key := os.Getenv("KMSKEY")
	locationID := "global"

	ctx := appengine.NewContext(r)
	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}

	kmsService, err := cloudkms.New(client)
	if err != nil {
		log.Fatal(err)
	}
	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, locationID, keyRing, key)
	req := &cloudkms.DecryptRequest{
		Ciphertext: ciphertext,
	}
	resp, err := kmsService.Projects.Locations.KeyRings.CryptoKeys.Decrypt(parentName, req).Do()
	if err != nil {
		log.Fatal(err)
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

func url_verificationHandler(w http.ResponseWriter, body []byte, r *http.Request) {
	var challengeRequest ChallengeRequest
	err := json.Unmarshal(body, &challengeRequest)
	if err != nil {
		log.Fatal(err)
	}
	if challengeRequest.Token == slackClientSecret(r) {
		fmt.Fprintln(w, challengeRequest.Challenge)
	}
}
func event_callbackHandler(w http.ResponseWriter, body []byte, r *http.Request) {
	var eventCallback EventCallback
	err := json.Unmarshal(body, &eventCallback)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(w, eventCallback)

}
func slackEventRouter(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Body.Close()
	eventType := result["type"]
	switch eventType {
	case "url_verification":
		url_verificationHandler(w, body, r)
	case "event_callback":
		event_callbackHandler(w, body, r)
	default:
	}
}
