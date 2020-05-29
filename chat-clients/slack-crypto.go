package main

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"crypto/hmac"

	"google.golang.org/api/cloudkms/v1"
)

//Decrypt decrypts ciphertext with Google Cloud KMS and returns plaintext
func (c *SlackChatClient) Decrypt(ctx context.Context, keyName string, ciphertext string) (string, error) {

	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		c.config.projectID, c.config.kmsSlackKeyLocation, c.config.kmsKeyring, keyName)
	req := &cloudkms.DecryptRequest{
		Ciphertext: ciphertext,
	}
	// c.log.Debugf("decrypting ciphertext: %s", ciphertext)
	resp, err := c.ecm.kmsClient.Projects.Locations.KeyRings.CryptoKeys.Decrypt(parentName, req).Do()
	if err != nil {
		return "", err
	}

	decodedString, _ := base64.StdEncoding.DecodeString(resp.Plaintext)
	// c.log.Debugf("decoded plaintext: %s\nundecoded plaintext: %s\n", string(decodedString), resp.Plaintext)
	return string(decodedString), nil
}

//Encrypt encrypts plaintext with Google Cloud KMS and returns ciphertext
func (c *SlackChatClient) Encrypt(ctx context.Context, keyName string, plaintext string) (string, error) {

	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		c.config.projectID, c.config.kmsSlackKeyLocation, c.config.kmsKeyring, keyName)
	encodedPlaintext := base64.StdEncoding.EncodeToString([]byte(plaintext))
	req := &cloudkms.EncryptRequest{
		Plaintext: encodedPlaintext,
	}
	resp, err := c.ecm.kmsClient.Projects.Locations.KeyRings.CryptoKeys.Encrypt(parentName, req).Do()
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
