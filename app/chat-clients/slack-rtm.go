package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/serialx/hashring"

	"github.com/aasmall/dicemagic/app/dicelang/errors"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

func SlackRTMInitCtx(ctx context.Context, safeDoc *tsSlackInstallInstanceDoc, env *env) {
	safeDoc.mux.Lock()
	defer safeDoc.mux.Unlock()
	installDoc := safeDoc.doc
	env.log.Debugf("Setting up RTM to Slack for: %s", installDoc.TeamName)
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
		env.log.Debugf("Could not set doc to Open: %+v\n", err)
		return
	}
	go rtm.ManageConnection()

	for msg := range rtm.IncomingEvents {
		fmt.Print("Event Received: ")
		err := updateSlackInstanceStatusLastSeen(ctx, env, env.config.podName, installDoc.Key.Parent, true)
		if err != nil {
			env.log.Debugf("Could not set doc to Open: %+v\n", err)
			return
		}
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			env.log.Debugf("slack.ConnectedEvent Infos: %v", ev.Info)
			env.log.Debugf("Connection counter: %v", ev.ConnectionCount)
		case *slack.MessageEvent:
			env.log.Debugf("message: %+v\n", ev)
			if DetectDM(ctx, slackApi, ev.Channel) && ev.SubType != "bot_message" {
				env.log.Debugf("\n\nIS DM: %s\n", ev.Text)
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

		case *slack.RTMError:
			env.log.Errorf("RTM Error: %s", ev.Error())

		case *slack.InvalidAuthEvent:
			rtm.Disconnect()
			delete(env.openRTMConnections, installDoc.TeamID)
			env.log.Debug("Invalid credentials\n")
			err := updateSlackInstanceStatusLastSeen(ctx, env, env.config.podName, installDoc.Key.Parent, false)
			if err != nil {
				env.log.Debugf("Could not set doc to Closed: %+v\n", err)
				return
			}
			err = deleteSlackInstallInstance(ctx, env, installDoc.Key)
			if err != nil {
				env.log.Debugf("error deleting install instance: %s", err)
			}
			break

		default:
		}
	}
	err = updateSlackInstanceStatusLastSeen(ctx, env, env.config.podName, installDoc.Key.Parent, false)
	if err != nil {
		env.log.Debugf("Could not set status to closed: %+v\n", err)
		return
	}
	fmt.Print("done")
}

func ManageSlackConnections(ctx context.Context, env *env) {
	for {
		docChan, errc := getAllSlackInstallInstances(ctx, env)
		select {
		case err := <-errc:
			if err != nil {
				env.log.Errorf("new error in channel: %+v", err)
			}
		case res := <-docChan:
			if res != nil {
				if res.doc != nil {
					env.log.Debugf("new instance in channel: %+v\n", res)
					go SlackRTMInitCtx(ctx, res, env)
				}
			}
		}
	}
	time.Sleep(time.Second * 5)
}
func RebalancePods(ctx context.Context, env *env) {
	for {
		// Create a ring hash and assign all teams to pods
		teams, err := getAllTeams(ctx, env)
		if err != nil {
			env.log.Criticalf("Failed to get Team Keys: %v", err)
			continue
		}
		pods := GetPods(env)
		ring := hashring.New(pods)

		for team := range teams {
			pod, ok := ring.GetNode(team)
			if !ok {
				env.log.Criticalf("Failed to get pod(%s) for team(%s) ", pod, team)
				continue
			}
			err := AssignTeamToPod(ctx, env, teams[team], pod)
			if err != nil {
				env.log.Criticalf("Failed to assign team(%s) to pod(%s): %v", team, pod, err)
				continue
			}
		}

		// kill connections that aren't on the correct pod.
		for team, rtm := range env.openRTMConnections {
			statusDocs, err := getAllStatusForTeamID(ctx, env, team)
			if err != nil {
				log.Fatalf("unable to get Slack Install Instance by Team: %s", team)
				break
			}
			if len(statusDocs) != 1 {
				log.Fatalf("multiple status docs for team: %s", team)
				break
			}
			if statusDocs[0].Pod != env.config.podName {
				rtm.Disconnect()
				delete(env.openRTMConnections, team)
			}
		}
		time.Sleep(time.Second * 5)
	}
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
