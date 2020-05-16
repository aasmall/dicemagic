package main

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"google.golang.org/api/option"

	"crypto/hmac"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
)

func (c *SlackChatClient) Decrypt(ctx context.Context, keyName string, ciphertext string) (string, error) {

	var kmsService *cloudkms.Service
	var err error

	if c.config.local {
		kmsService, err = cloudkms.NewService(ctx,
			option.WithEndpoint(c.config.mockKMSURL),
			option.WithAPIKey("mockAPIKey"),
			option.WithHTTPClient(c.httpClient))
	} else {
		kmsService, err = cloudkms.NewService(ctx,
			option.WithHTTPClient(c.httpClient))
	}
	if err != nil {
		return "", fmt.Errorf("Error creating cloudKMS.service: %v", err)
	}

	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		c.config.projectID, c.config.kmsSlackKeyLocation, c.config.kmsKeyring, keyName)
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
func (c *SlackChatClient) Encrypt(ctx context.Context, keyName string, plaintext string) (string, error) {

	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		return "", err
	}

	kmsService, err := cloudkms.New(client)

	if err != nil {
		return "", err
	}

	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		c.config.projectID, c.config.kmsSlackKeyLocation, c.config.kmsKeyring, keyName)
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

// CalculateHMAC creats an cryptographically correct byte slice for data and a secret
func CalculateHMAC(secret string, data []byte) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return h.Sum(nil)
}
