package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/serialx/hashring"

	"github.com/aasmall/dicemagic/app/dicelang/errors"
	pb "github.com/aasmall/dicemagic/app/proto"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

const commandRegex = `(?mi)^!(\w+)\b\s*=[\t|\f|\v| ]*(.*)$`

func (c *SlackChatClient) Disconnect(id int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.DisconnectNoLock(id)
}

func (c *SlackChatClient) DisconnectNoLock(id int) error {
	if c.slackConnectionPool[id] != nil {
		err := c.slackConnectionPool[id].conn.Disconnect()
		if err != nil {
			return err
		}
		delete(c.slackConnectionPool, id)
	}
	return nil
}
func (c *SlackChatClient) DisconnectIfUnassigned(assignedTeamIDs []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, conn := range c.slackConnectionPool {
		if !stringInSlice(conn.teamID, assignedTeamIDs) {
			err := c.DisconnectNoLock(conn.ID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *SlackChatClient) OpenConnection(ctx context.Context, teamID string) error {
	c.mu.Lock()
	c.log.Debugf("Setting up RTM to Slack for: %s", teamID)
	for _, conn := range c.slackConnectionPool {
		if conn.teamID == teamID {
			// connection already open
			defer c.mu.Unlock()
			return nil
		}
	}

	installDoc, err := c.GetFirstSlackInstallInstanceByTeamID(ctx, teamID)
	if err != nil {
		c.log.Errorf("Could not get slack install instance for team(%s): %s", teamID, err)
		defer c.mu.Unlock()
		return err
	}

	botAccessToken, err := c.Decrypt(ctx, c.config.kmsSlackKey, installDoc.Bot.EncBotAccessToken)
	if err != nil {
		c.log.Critical("could not decrypt access token")
		defer c.mu.Unlock()
		return err
	}
	connectionInfo := &SlackConnection{
		teamID:      teamID,
		botID:       installDoc.Bot.BotUserID,
		ID:          c.idGen.Next(),
		oAuthDocKey: installDoc.Key,
		client: slack.New(
			botAccessToken,
			slack.OptionDebug(c.config.debug),
			slack.OptionLog(c.log),
		),
	}
	connectionInfo.conn = connectionInfo.client.NewRTM()

	c.slackConnectionPool[connectionInfo.ID] = connectionInfo
	c.mu.Unlock()
	go func() {
		c.listen(ctx, connectionInfo)
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.slackConnectionPool, connectionInfo.ID)
	}()
	return nil
}

func (c *SlackChatClient) listen(ctx context.Context, connectionInfo *SlackConnection) error {
	var err error
	saveCommand := regexp.MustCompile(`(?i)^!(?P<name>\w+)\b\s*=[\t|\f|\v| ]*(?P<cmd>.*)$`)
	execCommand := regexp.MustCompile(`(?i)^!(?P<name>\w+)\b\s*$`)
	go connectionInfo.conn.ManageConnection()
	for msg := range connectionInfo.conn.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			c.log.Debugf("slack.ConnectedEvent Infos: %v", ev.Info)
			c.log.Debugf("Connection counter: %v", ev.ConnectionCount)

		case *slack.MessageEvent:
			mention, cmd := c.IsMention(ev.Text, connectionInfo.botID)
			c.log.Debugf("message: %+v\nmention: %+v\ncmd: %+v\n", ev, mention, cmd)
			// Don't respond to self or other bots
			if ev.SubType == "bot_message" {
				continue
			}
			// If the channel type is DM, MultiDM, or standard but this bot was mentioned in it.
			if cType := c.GetChannelType(ctx, connectionInfo.client, ev.Team, ev.Channel); cType == DM || cType == MultiDM || (cType == Standard && mention) {
				//if the command starts with bang, check if it matches known command regexes
				if strings.HasPrefix(cmd, "!") {
					switch {
					case cmd == "!!":
						cmd, err = c.GetLastCommand(ev.User, ev.Team)
						if err != nil {
							continue
						}
						c.Reply(connectionInfo, cmd, ev.Channel)
						break
					case saveCommand.MatchString(cmd):
						saveCommandMap := regexToMap(saveCommand, cmd)
						c.log.Debugf("Save: %s", saveCommandMap)
						err = c.SaveCommand(ev.User, ev.Team, saveCommandMap)
						if err != nil {
							continue
						}
						connectionInfo.client.PostMessage(ev.Channel, slack.MsgOptionText(fmt.Sprintf(`Command "%s" saved: %s`, saveCommandMap["name"], saveCommandMap["cmd"]), false))
					case execCommand.MatchString(cmd):
						execCommandMap := regexToMap(execCommand, cmd)
						c.log.Debugf("Exec: %s", execCommandMap)
						cmd, err = c.GetCommand(ev.User, ev.Team, execCommandMap)
						if err != nil {
							continue
						}
						c.Reply(connectionInfo, cmd, ev.Channel)
					default:
						connectionInfo.client.PostMessage(ev.Channel, slack.MsgOptionText("Unrecognized command.", false))

					}
				} else {
					c.Reply(connectionInfo, cmd, ev.Channel)
					c.SetLastCommand(ev.User, ev.Team, cmd)
				}
			}
		case *slack.RTMError:
			c.log.Errorf("RTM Error: %s", ev.Error())

		case *slack.InvalidAuthEvent:
			c.log.Debug("Invalid credentials. Disconnecting. \n")
			err := c.Disconnect(connectionInfo.ID)
			if err != nil {
				return err
			}
			c.log.Debug("Invalid credentials. Deleting oAuthRecord. \n")
			return c.DeleteSlackInstallInstance(ctx, connectionInfo.oAuthDocKey)
		default:

		}
	}
	return nil
}

func regexToMap(re *regexp.Regexp, input string) map[string]string {
	match := re.FindStringSubmatch(input)
	result := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}
	return result
}

func (c *SlackChatClient) IsMention(text string, botID string) (bool, string) {
	formattedBotID := fmt.Sprintf("<@%s>", botID)
	if strings.Contains(text, formattedBotID) {
		return true, strings.TrimSpace(strings.SplitAfter(text, formattedBotID)[1])
	}
	return false, strings.TrimSpace(text)
}

func (c *SlackChatClient) Reply(conn *SlackConnection, cmd string, channel string) {
	var rollResponse *pb.RollResponse
	var err error
	rollResponse, err = Roll(c.diceClient, cmd)
	if err != nil {
		c.log.Errorf("Unexpected error: %+v", err)
		conn.client.PostMessage(channel, slack.MsgOptionText(fmt.Sprintf("Oops! an unexpected error occured: %s", err), false))
		return
	}
	if !rollResponse.Ok {
		if rollResponse.Error.Code == errors.Friendly {
			conn.client.PostMessage(channel, slack.MsgOptionText(rollResponse.Error.Msg, false))
			return
		} else {
			conn.client.PostMessage(channel, slack.MsgOptionText(fmt.Sprintf("Oops! an error occured: %s", rollResponse.Error.Msg), false))
			return
		}
	}
	attachments := SlackAttachmentsFromRollResponse(rollResponse)
	conn.client.PostMessage(channel, slack.MsgOptionAttachments(attachments...))
}
func (c *SlackChatClient) ManageSlackConnections(ctx context.Context, freq time.Duration) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()
	for range ticker.C {
		if c.ShuttingDown {
			return
		}
		teams, err := c.GetTeamsAssignedtoPod()
		if err != nil {
			c.log.Errorf("could not retrive teams assigned to pod(%s): %v", c.config.podName, err)
			continue
		}
		c.log.Debugf("teams assigned to me: %s", teams)
		for _, teamID := range teams {
			err := c.OpenConnection(ctx, teamID)
			if err != nil {
				c.log.Errorf("could not establish Slack RTM connection for TeamID(%s): %s", teamID, err)
				continue
			}
		}
		err = c.DisconnectIfUnassigned(teams)
		if err != nil {
			c.log.Errorf("could not disconnect: %s", err)
			continue
		}
	}
}

func (c *SlackChatClient) ManagePods(ctx context.Context, freq time.Duration) {
	go func() {
		c.RebalancePods(ctx, freq*2)
		ticker := time.NewTicker(freq)
		defer ticker.Stop()
		for range ticker.C {
			if c.ShuttingDown {
				return
			}
			c.RebalancePods(ctx, freq*2)
		}
	}()
}
func (c *SlackChatClient) RebalancePods(ctx context.Context, assignmentExpiry time.Duration) {
	// Create a ring hash and assign all teams to pods
	teams, _ := c.GetHashKeys("teams")
	pods, _ := c.GetHashKeys("pods")
	ring := hashring.New(pods)
	for _, team := range teams {
		pod, ok := ring.GetNode(team)
		if !ok {
			c.log.Criticalf("Failed to hash pod(%s) for team(%s) ", pod, team)
			continue
		}
		c.log.Debugf("assigning pod(%s) to teamID (%s)", pod, team)
		err := c.AssignTeamToPod(team, pod, assignmentExpiry)
		if err != nil {
			c.log.Criticalf("Failed to assign team(%s) to pod(%s): %v", team, pod, err)
			continue
		}
	}
}

func (c *SlackChatClient) GetChannelType(ctx context.Context, slackAPI *slack.Client, teamID string, channel string) ChannelType {
	cType, err := c.GetCachedChannelType(teamID, channel)
	fmt.Printf("Channel Type is: %s\n", cType.String())
	if err != nil || cType == Unknown {
		fmt.Printf("\n\nGoing to database for channel type\n\n")
		_, chanErr := slackAPI.GetChannelInfoContext(ctx, channel)
		_, groupErr := slackAPI.GetGroupInfoContext(ctx, channel)
		if chanErr != nil && groupErr != nil {
			cType = DM
		} else if chanErr != nil {
			cType = MultiDM
		} else {
			cType = Standard
		}
	}
	//go c.SilentSetCachedChannelType(teamID, channel, cType)
	err = c.SetCachedChannelType(teamID, channel, &cType)
	if err != nil {
		panic(err)
	}
	return cType

}
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
