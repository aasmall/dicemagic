package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"google.golang.org/appengine/aetest"
	yaml "gopkg.in/yaml.v2"
)

var ctx context.Context

func TestMain(m *testing.M) {
	var appYaml AppYaml
	appYaml.SetEnvironmentVariables()
	localctx, done, _ := aetest.NewContext()
	ctx = localctx
	defer done()
	os.Exit(m.Run())
}

func (c *AppYaml) getAppYaml() *AppYaml {

	yamlFile, err := ioutil.ReadFile("app.yaml")
	if err != nil {
		fmt.Printf("yamlFile.Get err   #%v \n\n", err)
		return nil
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		fmt.Printf("Unmarshal: %v\n\n", err)
		return nil
	}

	return c
}
func (c *AppYaml) SetEnvironmentVariables() *AppYaml {
	c.getAppYaml()
	os.Setenv("PROJECT_ID", c.AppYamlEnvVariables.PROJECTID)
	os.Setenv("KMSKEYRING", c.AppYamlEnvVariables.KMSKEYRING)
	os.Setenv("KMSKEY", c.AppYamlEnvVariables.KMSKEY)
	return c
}

func TestEncryptDecrypt(t *testing.T) {
	testString := "EncryptThis"
	fmt.Printf("testString: %v\n", testString)
	ciphertext, err := encrypt(ctx, testString)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("ciphertext: %v\n", ciphertext)
	plaintext, err := decrypt(ctx, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("plaintext: %v\n", plaintext)
	if !(plaintext == testString) {
		t.Fatalf("plaintext does not match testString: %+v", plaintext)
	}
}

func TestParser(t *testing.T) {
	text := "ROLL 1d4+100(mundane)+1d4+1000(fire)"
	stmt, _ := NewParser(strings.NewReader(text)).Parse()
	fmt.Printf("stmt: %v\n", stmt)
}
