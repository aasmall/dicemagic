package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aasmall/dicemagic/app/dicelang/errors"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

func SlackRTMInitCtx(ctx context.Context, safeDoc *tsSlackInstallInstanceDoc, env *env) {
	safeDoc.mux.Lock()
	defer safeDoc.mux.Unlock()
	installDoc := safeDoc.doc
	log.Printf("Setting up RTM to Slack for: %s", installDoc.TeamName)
	botAccessToken, err := decrypt(ctx, env, installDoc.Bot.EncBotAccessToken)
	if err != nil {
		log.Panic("could not decrypt access token")
	}
	slackApi := slack.New(
		botAccessToken,
		slack.OptionDebug(true),
		slack.OptionLog(log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags)),
	)
	rtm := slackApi.NewRTM()
	env.openRTMConnections[installDoc.TeamID] = rtm

	err = updateSlackInstanceStatusLastSeen(ctx, env, env.config.podName, installDoc.Key.Parent, true)
	if err != nil {
		fmt.Println("Could not set doc to Open: %+v\n", err)
		return
	}
	go rtm.ManageConnection()

	for msg := range rtm.IncomingEvents {
		fmt.Print("Event Received: ")
		err := updateSlackInstanceStatusLastSeen(ctx, env, env.config.podName, installDoc.Key.Parent, true)
		if err != nil {
			fmt.Println("Could not set doc to Open: %+v\n", err)
			return
		}
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			fmt.Println("Infos:", ev.Info)
			fmt.Println("Connection counter:", ev.ConnectionCount)
			// Replace C2147483705 with your Channel ID
			c, err := slackApi.GetChannelInfoContext(ctx, "CDKK8HPL7")
			if err != nil {
				fmt.Printf("error getting channel info: %s\n", err)
				rtm.Disconnect()
				return
			}
			fmt.Printf("Channel: %v\n", c)
			rtm.SendMessage(rtm.NewOutgoingMessage("Hello world", c.ID))

		case *slack.MessageEvent:
			fmt.Printf("message: %+v\n", ev)
			if DetectDM(ctx, slackApi, ev.Channel) && ev.SubType != "bot_message" {
				fmt.Printf("\n\nIS DM: %s\n", ev.Text)
				rollResponse, err := Roll(env.diceServerClient, ev.Text)
				if err != nil {
					env.log.Errorf("Unexpected error: %+v", err)
					slackApi.PostMessage(ev.Channel, slack.MsgOptionText(fmt.Sprintf("Oops! an unexpected error occured: %s", err), false))
					break
				}

				if !rollResponse.Ok {
					if rollResponse.Error.Code == errors.Friendly {
						slackApi.PostMessage(ev.Channel, slack.MsgOptionText(rollResponse.Error.Msg, false))
						break
					} else {
						slackApi.PostMessage(ev.Channel, slack.MsgOptionText(fmt.Sprintf("Oops! an error occured: %s", rollResponse.Error.Msg), false))
						break

					}
				}

				attachment := SlackAttachmentFromRollResponse(rollResponse.DiceSet)
				slackApi.PostMessage(ev.Channel, slack.MsgOptionAttachments(attachment))
			}

		case *slack.PresenceChangeEvent:
			fmt.Printf("Presence Change: %v\n", ev)

		case *slack.LatencyReport:
			fmt.Printf("Current latency: %v\n", ev.Value)

		case *slack.RTMError:
			fmt.Printf("Error: %s\n", ev.Error())

		case *slack.InvalidAuthEvent:
			rtm.Disconnect()
			delete(env.openRTMConnections, installDoc.TeamID)
			fmt.Printf("Invalid credentials\n")
			err := updateSlackInstanceStatusLastSeen(ctx, env, env.config.podName, installDoc.Key.Parent, false)
			if err != nil {
				fmt.Printf("Could not set doc to Closed: %+v\n", err)
				return
			}
			err = deleteSlackInstallInstance(ctx, env, installDoc)
			if err != nil {
				fmt.Printf("error deleting install instance: %s", err)
			}
			break

		default:

			// Ignore other events..
			// fmt.Printf("Unexpected: %v\n", msg.Data)
		}
	}
	err = updateSlackInstanceStatusLastSeen(ctx, env, env.config.podName, installDoc.Key.Parent, false)
	if err != nil {
		fmt.Println("Could not set status to closed: %+v\n", err)
		return
	}
	fmt.Print("done")
}

func DetectDM(ctx context.Context, slackAPI *slack.Client, channel string) bool {
	_, chanErr := slackAPI.GetChannelInfoContext(ctx, channel)
	_, groupErr := slackAPI.GetGroupInfoContext(ctx, channel)
	if chanErr != nil {
		if groupErr != nil {
			return true
		}
	}
	return false
}
