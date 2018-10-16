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
	"net/http"
	"strconv"
	"strings"
	"time"

	"crypto/hmac"

	"github.com/labstack/gommon/log"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
)

func decrypt(ctx context.Context, env *env, ciphertext string) (string, error) {

	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		return "", err
	}

	kmsService, err := cloudkms.New(client)
	if err != nil {
		return "", err
	}

	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		env.config.projectID, env.config.kmsSlackKeyLocation, env.config.kmsKeyring, env.config.kmsSlackKey)
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
func encrypt(ctx context.Context, env *env, plaintext string) (string, error) {

	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		return "", err
	}

	kmsService, err := cloudkms.New(client)
	if err != nil {
		return "", err
	}

	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		env.config.projectID, env.config.kmsSlackKeyLocation, env.config.kmsKeyring, env.config.kmsSlackKey)
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

func ValidateSlackSignature(env *env, r *http.Request) bool {
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

	decSigningSecret, err := decrypt(r.Context(), env, env.config.encSlackSigningSecret)
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
