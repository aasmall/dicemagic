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
