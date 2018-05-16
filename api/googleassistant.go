package api

import (
	"bytes"
	"fmt"

	"github.com/aasmall/dicemagic/lib"
)

//AssistantResponse represents a response that will be sent to Dialogflow for Google Actions API
type AssistantResponse struct {
	ExpectUserResponse bool          `json:"expectUserResponse,omitempty"`
	IsSsml             bool          `json:"isSsml,omitempty"`
	NoInputPrompts     []interface{} `json:"noInputPrompts,omitempty"`
	RichResponse       `json:"richResponse,omitempty"`
}
type RichResponse struct {
	Items       []interface{} `json:"items,omitempty"`
	Suggestions []struct {
		Title string `json:"title,omitempty"`
	} `json:"suggestions,omitempty"`
}
type SimpleResponseItem struct {
	SimpleResponse struct {
		TextToSpeech string `json:"textToSpeech"`
		DisplayText  string `json:"displayText"`
	} `json:"simpleResponse,omitempty"`
}

type BasicCardItem struct {
	BasicCard struct {
		Title         string `json:"title,omitempty"`
		FormattedText string `json:"formattedText,omitempty"`
	} `json:"basicCard,omitempty"`
}

func createMarkdownDamageString(rollTotals []lib.RollTotal) (string, int64) {
	var buffer bytes.Buffer
	t := int64(0)
	for _, e := range rollTotals {
		if e.RollType == "" {
			buffer.WriteString(fmt.Sprintf("__%d__  \n", e.RollResult))
		} else {
			buffer.WriteString(fmt.Sprintf("%s: __%d__  \n", e.RollType, e.RollResult))
		}
		t += e.RollResult
	}
	return buffer.String(), t
}
func rollExpressionToMarkdown(expression *lib.RollExpression) (string, int64, error) {
	rollTotals, err := expression.GetTotalsByType()
	if err != nil {
		return "", 0, err
	}
	s, t := createMarkdownDamageString(rollTotals)
	return s, t, nil
}
