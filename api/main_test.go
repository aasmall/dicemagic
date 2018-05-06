package api

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

type AppYaml struct {
	Runtime             string              `yaml:"runtime"`
	APIVersion          string              `yaml:"api_version"`
	AppYamlHandlers     []AppYamlHandlers   `yaml:"handlers"`
	AppYamlEnvVariables AppYamlEnvVariables `yaml:"env_variables"`
}

type AppYamlHandlers struct {
	URL         string `yaml:"url"`
	StaticFiles string `yaml:"static_files,omitempty"`
	Upload      string `yaml:"upload,omitempty"`
	Script      string `yaml:"script,omitempty"`
}
type AppYamlEnvVariables struct {
	SLACKKEY               string `yaml:"SLACK_KEY"`
	SLACKCLIENTSECRET      string `yaml:"SLACK_CLIENT_SECRET"`
	PROJECTID              string `yaml:"PROJECT_ID"`
	KMSKEYRING             string `yaml:"KMSKEYRING"`
	KMSKEY                 string `yaml:"KMSKEY"`
	SLACKBOTUSERACCESTOKEN string `yaml:"SLACK_BOT_USER_ACCES_TOKEN"`
}

func TestMain(m *testing.M) {
	var appYaml AppYaml
	appYaml.SetEnvironmentVariables()

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
