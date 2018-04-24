package main

type EventCallback struct {
	Token       string `json:"token"`
	TeamID      string `json:"team_id"`
	APIAppID    string `json:"api_app_id"`
	InnerEvent  `json:"event"`
	Type        string   `json:"type"`
	EventID     string   `json:"event_id"`
	EventTime   int64    `json:"event_time"`
	AuthedUsers []string `json:"authed_users"`
}
type InnerEvent struct {
	Type    string `json:"type"`
	User    string `json:"user"`
	Text    string `json:"text"`
	Ts      string `json:"ts"`
	Channel string `json:"channel"`
	EventTs string `json:"event_ts"`
}
type ChallengeRequest struct {
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
	Type      string `json:"type"`
}

type KMSDecryptResponse struct {
	plaintext string `json:"plaintext"`
}
type KMSEncryptResponse struct {
	ciphertext string `json:"ciphertext"`
}
type Message struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}
type OAuthApprovalResponse struct {
	AccessToken     string `json:"access_token"`
	Scope           string `json:"scope"`
	TeamName        string `json:"team_name"`
	TeamID          string `json:"team_id"`
	IncomingWebhook `json:"incoming_webhook"`
	Bot             `json:"bot"`
}
type OauthAccessRequest struct {
	ClientID     string
	ClientSecret string
	Code         string
	State        string
}
type IncomingWebhook struct {
	URL              string `json:"url"`
	Channel          string `json:"channel"`
	ConfigurationURL string `json:"configuration_url"`
}
type Bot struct {
	BotUserID      string `json:"bot_user_id"`
	BotAccessToken string `json:"bot_access_token"`
}
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
