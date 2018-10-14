package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

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
func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
