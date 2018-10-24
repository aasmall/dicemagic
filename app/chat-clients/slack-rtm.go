package main

import (
	"fmt"
	"time"

	"github.com/serialx/hashring"

	"github.com/aasmall/dicemagic/app/dicelang/errors"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

func SlackRTMInitCtx(ctx context.Context, installDoc *SlackInstallInstanceDoc, env *env) {
	env.log.Debugf("Setting up RTM to Slack for: %s", installDoc.TeamName)
	botAccessToken, err := decrypt(ctx, env, installDoc.Bot.EncBotAccessToken)
	if err != nil {
		env.log.Critical("could not decrypt access token")
		return
	}
	slackApi := slack.New(
		botAccessToken,
		slack.OptionDebug(env.config.debug),
		slack.OptionLog(env.log),
	)
	rtm := slackApi.NewRTM()
	env.openRTMConnections[installDoc.TeamID] = rtm
	defer delete(env.openRTMConnections, installDoc.TeamID)
	go rtm.ManageConnection()
	for msg := range rtm.IncomingEvents {
		fmt.Print("Event Received: ")
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
				attachments := SlackAttachmentsFromRollResponse(rollResponse)
				slackApi.PostMessage(ev.Channel, slack.MsgOptionAttachments(attachments...))
			}

		case *slack.RTMError:
			env.log.Errorf("RTM Error: %s", ev.Error())

		case *slack.InvalidAuthEvent:
			rtm.Disconnect()
			env.log.Debug("Invalid credentials\n")
			err = deleteSlackInstallInstance(ctx, env, installDoc.Key)
			if err != nil {
				env.log.Debugf("error deleting install instance: %s", err)
			}
			return

		default:
		}
	}
}

func (env *env) ManageSlackConnections(ctx context.Context, freq time.Duration) {
	go func() {
		ticker := time.NewTicker(freq)
		defer ticker.Stop()
		for range ticker.C {
			if env.ShuttingDown {
				return
			}
			teams, err := env.GetTeamsAssignedtoPod()
			if err != nil {
				env.log.Errorf("could not retrive teams assigned to pod(%s): %v", env.config.podName, err)
				continue
			}
			fmt.Println("teams: ", teams)
			for _, teamID := range teams {
				if env.openRTMConnections[teamID] == nil {
					err := env.EstablishSlackRTMSocket(ctx, teamID)
					if err != nil {
						env.log.Errorf("could not establish Slack RTM connection for TeamID(%s): %s", teamID, err)
						continue
					}
				}
			}
			for k, v := range env.openRTMConnections {
				if !stringInSlice(k, teams) {
					v.Disconnect()
					delete(env.openRTMConnections, k)
				}
			}
		}
	}()
}

func (env *env) EstablishSlackRTMSocket(ctx context.Context, teamID string) error {
	installDoc, err := GetFirstSlackInstallInstanceByTeamID(ctx, env, teamID)
	if err != nil {
		env.log.Errorf("Could not get slack install instance for team(%s): %s", teamID, err)
		return err
	}
	go SlackRTMInitCtx(ctx, installDoc, env)
	return nil
}

func (env *env) ManagePods(ctx context.Context, freq time.Duration) {
	go func() {
		ticker := time.NewTicker(freq)
		defer ticker.Stop()
		for range ticker.C {
			if env.ShuttingDown {
				return
			}
			env.RebalancePods(ctx, freq*2)
		}
	}()
}
func (env *env) RebalancePods(ctx context.Context, assignmentExpiry time.Duration) {
	// Create a ring hash and assign all teams to pods
	teams := GetHashKeys("teams", env)
	pods := GetHashKeys("pods", env)
	ring := hashring.New(pods)
	for _, team := range teams {
		pod, ok := ring.GetNode(team)
		if !ok {
			env.log.Criticalf("Failed to hash pod(%s) for team(%s) ", pod, team)
			continue
		}
		env.log.Debugf("assigning pod(%s) to teamID (%s)", pod, team)
		err := AssignTeamToPod(env, team, pod, assignmentExpiry)
		if err != nil {
			env.log.Criticalf("Failed to assign team(%s) to pod(%s): %v", team, pod, err)
			continue
		}
	}
	teamsAssigned, err := env.GetTeamsAssignedtoPod()
	if err != nil {
		env.log.Debugf("unable to get assigned teams by pod: %s", env.config.podName)
		return
	}
	// kill connections that aren't on the correct pod.
	for connectedTeamID, rtm := range env.openRTMConnections {
		if !stringInSlice(connectedTeamID, teamsAssigned) {
			rtm.Disconnect()
			delete(env.openRTMConnections, connectedTeamID)
		}
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
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
