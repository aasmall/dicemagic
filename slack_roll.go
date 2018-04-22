package main

import (
	"encoding/base64"
	//"encoding/json"
	"fmt"
	//"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/appengine"
	"log"
	"net/http"
	"os"
)

type slashPostBody struct {
	token           string
	team_id         string
	team_domain     string
	enterprise_id   string
	enterprise_name string
	channel_id      string
	channel_name    string
	user_id         string
	user_name       string
	command         string
	text            string
	response_url    string
	trigger_id      string
}
type KMSDecryptResponse struct {
	plaintext string `json:"plaintext"`
}

func slackKey(r *http.Request) string {

	encKey := os.Getenv("SLACK_KEY")
	projectID := os.Getenv("PROJECT_ID")
	keyRing := os.Getenv("KMSKEYRING")
	key := os.Getenv("KMSKEY")
	locationID := "global"

	ctx := appengine.NewContext(r)
	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}
	// Create the KMS client.
	kmsService, err := cloudkms.New(client)
	if err != nil {
		log.Fatal(err)
	}
	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, locationID, keyRing, key)
	req := &cloudkms.DecryptRequest{
		Ciphertext: encKey,
	}
	resp, err := kmsService.Projects.Locations.KeyRings.CryptoKeys.Decrypt(parentName, req).Do()
	if err != nil {
		return ""
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
