package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func main() {
	slackSigString := "v0=66152f729bcb3e1918eb3be139a25bc0b53a73b94a430fea0d17d05940ab4ed5"
	slackSigBytes, _ := hex.DecodeString(strings.Split(slackSigString, "v0=")[1])
	timestamp := "1539570615"
	signingSecret := ""
	body := "token=z8uhLAlJ26fcEXotaVXVu4aI&team_id=T18462BCP&team_domain=docusignanddragons&channel_id=DAA6PDKT2&channel_name=directmessage&user_id=U19JZLUDR&user_name=atlashugged&command=%2Froll&text=3d12&response_url=https%3A%2F%2Fhooks.slack.com%2Fcommands%2FT18462BCP%2F455929915123%2FlSDLEsocFniwK7X7QH9zG0pS&trigger_id=455929915155.42142079431.120a9f850fa2899c0c98c49002d0867a"
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, body)
	baseString = "v0:1539570615:token=z8uhLAlJ26fcEXotaVXVu4aI&team_id=T18462BCP&team_domain=docusignanddragons&channel_id=DAA6PDKT2&channel_name=directmessage&user_id=U19JZLUDR&user_name=atlashugged&command=%2Froll&text=3d12&response_url=https%3A%2F%2Fhooks.slack.com%2Fcommands%2FT18462BCP%2F455856725444%2FSWuKtBiIsmXDdvtlRFVyeQtD&trigger_id=457400917926.42142079431.8dcc3d7b2a229f58368ffaefd6ef4de7"
	baseHash := CalculateHMAC(signingSecret, []byte(baseString))

	match := hmac.Equal(slackSigBytes, baseHash)

	fmt.Printf("slackSig: %+v\n", hex.EncodeToString(slackSigBytes))
	fmt.Printf("basehash: %+v\n", hex.EncodeToString(baseHash))
	fmt.Print(match)
}

func CalculateHMAC(secret string, data []byte) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return h.Sum(nil)
}
