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
type Message struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}
type Approval struct {
	AccessToken     string `json:"access_token"`
	Scope           string `json:"scope"`
	TeamName        string `json:"team_name"`
	TeamID          string `json:"team_id"`
	IncomingWebhook `json:"incoming_webhook"`
	Bot             `json:"bot"`
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
