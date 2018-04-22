package main

type EventCallback struct {
	Token       string   `json:"token"`
	TeamID      string   `json:"team_id"`
	APIAppID    string   `json:"api_app_id"`
	Event       struct{} `json:"event"`
	Type        string   `json:"type"`
	EventID     string   `json:"event_id"`
	EventTime   int64    `json:"event_time"`
	AuthedUsers []string `json:"authed_users"`
}
type ChallengeRequest struct {
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
	Type      string `json:"type"`
}

type KMSDecryptResponse struct {
	plaintext string `json:"plaintext"`
}
