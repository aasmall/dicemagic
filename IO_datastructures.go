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

type DialogueFlowRequest struct {
	ResponseID                  string                  `json:"responseId"`
	Session                     string                  `json:"session"`
	QueryResult                 DialogueFlowQueryResult `json:"queryResult"`
	OriginalDetectIntentRequest map[string]interface{}
}
type DialogueFlowQueryResult struct {
	QueryText                string                 `json:"queryText"`
	Parameters               map[string]interface{} `json:"parameters"`
	AllRequiredParamsPresent bool                   `json:"allRequiredParamsPresent"`
	FulfillmentText          string                 `json:"fulfillmentText"`
	FulfillmentMessages      []struct {
		Text struct {
			Text []string `json:"text"`
		} `json:"text"`
	} `json:"fulfillmentMessages"`
	OutputContexts []struct {
		Name          string `json:"name"`
		LifespanCount int    `json:"lifespanCount"`
		Parameters    struct {
			Param string `json:"param"`
		} `json:"parameters"`
	} `json:"outputContexts"`
	Intent struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"intent"`
	IntentDetectionConfidence float64 `json:"intentDetectionConfidence"`
	DiagnosticInfo            struct {
	} `json:"diagnosticInfo"`
	LanguageCode string `json:"languageCode"`
}
type DialogueFlowParameter struct {
	name  string
	value string
}
type DialogueFlowResponse struct {
	FulfillmentText     string `json:"fulfillmentText"`
	FulfillmentMessages []struct {
		Card struct {
			Title    string `json:"title"`
			Subtitle string `json:"subtitle"`
			ImageURI string `json:"imageUri"`
			Buttons  []struct {
				Text     string `json:"text"`
				Postback string `json:"postback"`
			} `json:"buttons"`
		} `json:"card"`
	} `json:"fulfillmentMessages"`
	Source  string `json:"source"`
	Payload struct {
		Google struct {
			ExpectUserResponse bool `json:"expectUserResponse"`
			RichResponse       struct {
				Items []struct {
					SimpleResponse struct {
						TextToSpeech string `json:"textToSpeech"`
					} `json:"simpleResponse"`
				} `json:"items"`
			} `json:"richResponse"`
		} `json:"google"`
		Facebook struct {
			Text string `json:"text"`
		} `json:"facebook"`
		Slack struct {
			Text string `json:"text"`
		} `json:"slack"`
	} `json:"payload"`
	OutputContexts []struct {
		Name          string `json:"name"`
		LifespanCount int    `json:"lifespanCount"`
		Parameters    struct {
			Param string `json:"param"`
		} `json:"parameters"`
	} `json:"outputContexts"`
	FollowupEventInput struct {
		Name         string `json:"name"`
		LanguageCode string `json:"languageCode"`
		Parameters   struct {
			Param string `json:"param"`
		} `json:"parameters"`
	} `json:"followupEventInput"`
}
