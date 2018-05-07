package api

//AssistantResponse represents a response that will be sent to Dialogflow for Google Actions API
type AssistantResponse struct {
	ConversationToken  string `json:"conversationToken"`
	ExpectUserResponse bool   `json:"expectUserResponse"`
	ExpectedInputs     []struct {
		InputPrompt struct {
			RichInitialPrompt struct {
				Items []struct {
					SimpleResponse struct {
						TextToSpeech string `json:"textToSpeech"`
						DisplayText  string `json:"displayText"`
					} `json:"simpleResponse"`
				} `json:"items"`
				Suggestions []interface{} `json:"suggestions"`
			} `json:"richInitialPrompt"`
		} `json:"inputPrompt"`
		PossibleIntents []struct {
			Intent string `json:"intent"`
		} `json:"possibleIntents"`
	} `json:"expectedInputs"`
}
