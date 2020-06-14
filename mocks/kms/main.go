package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/aasmall/dicemagic/lib/handler"
	"github.com/gorilla/mux"
)

type kmsConfig struct {
	keyData []byte
}
type env struct {
	config *kmsConfig
}
type encryptRequest struct {
	Plaintext                   string `json:"plaintext"`
	AdditionalAuthenticatedData string `json:"additionalAuthenticatedData,omitempty"`
}
type encryptResponse struct {
	Name       string `json:"name"`
	Ciphertext string `json:"ciphertext"`
}
type decryptRequest struct {
	Ciphertext                  string `json:"ciphertext"`
	AdditionalAuthenticatedData string `json:"additionalAuthenticatedData,omitempty"`
}
type decryptResponse struct {
	Name      string `json:"name"`
	Plaintext string `json:"plaintext"`
}

func main() {
	tls := flag.Bool("tls", true, "enable TLS with mock cert?")
	cli := flag.Bool("cli", false, "run as cli, not server.")
	encryptText := flag.String("encrypt", "", "text to encrypt")
	decryptText := flag.String("decrypt", "", "text to decrypt")

	flag.Parse()

	kmsConfig := &kmsConfig{
		keyData: []byte{
			0x15, 0x7, 0x9c, 0x8f, 0x9, 0xe6, 0x30, 0x0,
			0x39, 0x1, 0x4d, 0x9c, 0xf0, 0x79, 0xd7, 0xcf,
			0xd5, 0x48, 0x39, 0x41, 0x86, 0xf2, 0xf4, 0x50,
			0xbd, 0xa3, 0xcc, 0x46, 0x49, 0x8c, 0xb1, 0xf0}}
	env := &env{config: kmsConfig}
	if *cli {
		if (*encryptText == "" && *decryptText == "") || *encryptText != "" && *decryptText != "" {
			fmt.Println("You must specify at least and only one of 'encrypt' and 'decrypt'")
			return
		}
		if *encryptText != "" {
			ciphertext, err := encrypt(kmsConfig.keyData, *encryptText)
			if err != nil {
				fmt.Printf("Error encrypting text: %v\n", err)
			}
			fmt.Println(ciphertext)
			return
		} else if *decryptText != "" {
			plaintext, err := decrypt(kmsConfig.keyData, *decryptText)
			if err != nil {
				fmt.Printf("Error decrypting text: %v\n", err)
			}
			fmt.Println(plaintext)
			return
		} else {
			fmt.Println("This isn't possible.")
			return
		}
	}
	r := mux.NewRouter()
	r.Handle("/v1/projects/{project-id}/locations/{location}/keyRings/{keyring-name}/cryptoKeys/{key-name}:encrypt", handler.Handler{Env: env, H: encryptHandler})
	r.Handle("/v1/projects/{project-id}/locations/{location}/keyRings/{keyring-name}/cryptoKeys/{key-name}:decrypt", handler.Handler{Env: env, H: decryptHandler})
	if *tls {
		log.Fatal(http.ListenAndServeTLS(":40080", "/etc/mock-tls/tls.crt", "/etc/mock-tls/tls.key", r))
	} else {
		log.Fatal(http.ListenAndServe(":40080", r))
	}
}

func encryptHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, _ := e.(*env)
	key := env.config.keyData
	req := &encryptRequest{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		log.Printf("Unexpected error decoding REST request: %+v", err)
		return err
	}
	ciphertext, err := encrypt(key, req.Plaintext)
	if err != nil {
		log.Printf("Unexpected error decoding performing encryption: %+v", err)
		return err
	}
	encryptResponse := &encryptResponse{
		Ciphertext: ciphertext,
		Name:       "",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(encryptResponse)
	return nil
}
func decryptHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, _ := e.(*env)
	key := env.config.keyData
	req := &decryptRequest{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		log.Fatalf("Unexpected error decoding REST request: %+v", err)
		return err
	}
	log.Printf("decrypting ciphertext: %s", req.Ciphertext)
	plaintext, err := decrypt(key, req.Ciphertext)
	log.Printf("plaintext: %s", plaintext)
	if err != nil {
		log.Fatalf("Unexpected error decoding performing decryption: %+v", err)
		return err
	}
	decryptResponse := &decryptResponse{
		Plaintext: plaintext,
		Name:      "",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(decryptResponse)
	return nil
}
func encrypt(key []byte, message string) (encmess string, err error) {
	plaintext := []byte(message)

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	cipherText := make([]byte, aes.BlockSize+len(plaintext))
	iv := cipherText[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], plaintext)

	//returns to base64 encoded string
	encmess = base64.URLEncoding.EncodeToString(cipherText)
	return
}
func decrypt(key []byte, securemess string) (decodedmess string, err error) {
	cipherText, err := base64.URLEncoding.DecodeString(securemess)
	if err != nil {
		return
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	if len(cipherText) < aes.BlockSize {
		err = errors.New("ciphertext block size is too short")
		return
	}

	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(cipherText, cipherText)

	decodedmess = string(cipherText)
	return
}
