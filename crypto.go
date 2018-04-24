package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/appengine/log"
	"os"
)

func slackOauthToken(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_KEY"), ctx)
}
func slackClientSecret(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_CLIENT_SECRET"), ctx)
}
func slackBotAccessToken(ctx context.Context, integration *Integration) string {
	encryptedBotAccessToken := integration.OAuthApprovalResponse.Bot.BotAccessToken
	return decrypt(encryptedBotAccessToken, ctx)
}
func slackVerificationToken(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_CLIENT_VERIFICATION_TOKEN"), ctx)
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
func encrypt(plaintext string, ctx context.Context) string {
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
	encodedPlaintext := base64.StdEncoding.EncodeToString([]byte(plaintext))
	req := &cloudkms.EncryptRequest{
		Plaintext: encodedPlaintext,
	}
	resp, err := kmsService.Projects.Locations.KeyRings.CryptoKeys.Encrypt(parentName, req).Do()
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	return string(resp.Ciphertext)
}
