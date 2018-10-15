package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"time"

	"crypto/hmac"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
)

func slackClientSecret(ctx context.Context) string {
	val, _ := decrypt(ctx, encSlackClientSecret)
	return val
}
func decrypt(ctx context.Context, ciphertext string) (string, error) {

	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		return "", err
	}

	kmsService, err := cloudkms.New(client)
	if err != nil {
		return "", err
	}

	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, locationID, keyRing, key)
	req := &cloudkms.DecryptRequest{
		Ciphertext: ciphertext,
	}
	resp, err := kmsService.Projects.Locations.KeyRings.CryptoKeys.Decrypt(parentName, req).Do()
	if err != nil {
		return "", err
	}

	decodedString, _ := base64.StdEncoding.DecodeString(resp.Plaintext)
	return string(decodedString), nil
}
func encrypt(ctx context.Context, plaintext string) (string, error) {
	projectID := os.Getenv("project-id")
	keyRing := os.Getenv("keyring")
	key := os.Getenv("slack-kms-key")
	locationID := os.Getenv("slack-kms-key-location-id")

	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		return "", err
	}

	kmsService, err := cloudkms.New(client)
	if err != nil {
		return "", err
	}

	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, locationID, keyRing, key)
	encodedPlaintext := base64.StdEncoding.EncodeToString([]byte(plaintext))
	req := &cloudkms.EncryptRequest{
		Plaintext: encodedPlaintext,
	}
	resp, err := kmsService.Projects.Locations.KeyRings.CryptoKeys.Encrypt(parentName, req).Do()
	if err != nil {
		return "", err
	}

	return string(resp.Ciphertext), nil
}

// HashStrings computes the MD5 hash of all input strings
func HashStrings(inputs ...string) string {
	h := md5.New()
	for _, input := range inputs {
		h.Write([]byte(input))
	}
	hexb := h.Sum(nil)
	return hex.EncodeToString(hexb)
}

func CalculateHMAC(secret string, data []byte) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return h.Sum(nil)
}

func ValidateSlackSignature(r *http.Request) bool {
	//read relevant headers
	slackSigString := r.Header.Get("X-Slack-Signature")
	remoteHMAC, _ := hex.DecodeString(strings.Split(slackSigString, "v0=")[1])
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")

	//read body and reset request
	body, err := ioutil.ReadAll(r.Body)
	log.Println("body: " + string(body))
	if err != nil {
		log.Println("cannot validate slack signature. Cannot read body")
		return false
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	// check time skew
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Printf("cannot validate slack signature. Cannot parse timestamp: %s", timestamp)
		return false
	}
	delta := time.Now().Sub(time.Unix(ts, 0))
	log.Printf("timeskew: (%s)", delta.String())
	if delta.Minutes() > 5 {
		log.Printf("cannot validate slack signature. Time skew > 5 minutes (%s)", delta.String())
		return false
	}

	decSigningSecret, err := decrypt(r.Context(), slackSigningSecret)
	if err != nil {
		log.Printf("cannot validate slack signature. can't decrypt signing secret: %s", err)
		return false
	}

	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	log.Printf("baseString: %s", baseString)
	locahHMAC := CalculateHMAC(decSigningSecret, []byte(baseString))
	log.Printf("remoteHMAC: (%+v)\nlocahHMAC: (%+v)", hex.EncodeToString(remoteHMAC), hex.EncodeToString(locahHMAC))
	if hmac.Equal(remoteHMAC, locahHMAC) {
		return true
	}

	return false
}

func dumpRequest(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Println("could not dump request")
		return
	}
	log.Printf("HTTP Request Dump: \n\n%s", string(dump))
}
