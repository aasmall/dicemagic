package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/serialx/hashring"
)

const timeFormat = "2006-01-02 15:04:05.999999999 -0700 MST"
const threeMonths = time.Minute * 131000 // not right, but close enough

//ChannelType is an enum of slack channel types
//go:generate stringer -type=ChannelType
type ChannelType uint16

// Possible slack channel types
const (
	Unknown ChannelType = iota
	DM
	MultiDM
	Standard
)

// MarshalBinary marshals a channelType into []byte
func (cType *ChannelType) MarshalBinary() ([]byte, error) {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint16(bytes, uint16(*cType))
	return bytes, nil
}

// UnmarshalBinary unmarshals a channelType into []byte
func (cType *ChannelType) UnmarshalBinary(data []byte) error {
	if len(data) != 4 {
		return fmt.Errorf("data wrong size")
	}
	*cType = ChannelType(binary.BigEndian.Uint16(data))
	return nil
}

// SetLastCommand stores the last command issued with the "!!" token
func (c *SlackChatClient) SetLastCommand(userID string, teamID string, cmd string) error {
	key := fmt.Sprintf("command:%s:%s:!!", teamID, userID)
	return c.ecm.redisClient.Set(key, cmd, threeMonths).Err()
}

// GetLastCommand returns the command stores with the "!!" token
func (c *SlackChatClient) GetLastCommand(userID string, teamID string) (string, error) {
	key := fmt.Sprintf("command:%s:%s:!!", teamID, userID)
	cmd, err := c.ecm.redisClient.Get(key).Result()
	if err == nil {
		go c.SetLastCommand(userID, teamID, cmd)
	}
	return cmd, err
}

// SaveCommand saves a command with the given token and persists it to the datastore
func (c *SlackChatClient) SaveCommand(userID string, teamID string, commandMap map[string]string) error {
	key := fmt.Sprintf("command:%s:%s:%s", teamID, userID, commandMap["name"])
	go c.SlackDatastoreClient.UpsetRedisCommand(context.Background(),
		&RedisCommand{TeamID: teamID, UserID: userID, CommandKey: commandMap["name"], CommandValue: commandMap["cmd"], Expire: time.Now().Add(threeMonths)},
		c.config.slackAppID)
	return c.ecm.redisClient.Set(key, commandMap["cmd"], threeMonths).Err()
}

// GetCommand retrieves a command from redis with the given token in the form of map[CommandName]CommandValue
func (c *SlackChatClient) GetCommand(userID string, teamID string, commandMap map[string]string) (string, error) {
	key := fmt.Sprintf("command:%s:%s:%s", teamID, userID, commandMap["name"])
	cmd, err := c.ecm.redisClient.Get(key).Result()
	if err != nil {
		var dsCmd *RedisCommand
		dsCmd, err = c.SlackDatastoreClient.GetRedisCommand(context.Background(), teamID, userID, commandMap["name"], c.config.slackAppID)
		cmd = dsCmd.CommandValue
	}
	if err == nil {
		go c.SetLastCommand(userID, teamID, cmd)
		go c.SaveCommand(userID, teamID, map[string]string{"name": commandMap["name"], "cmd": cmd})
	}
	return cmd, err
}

// GetCachedChannelType retrieves a channel type from redis for a given channel string
func (c *SlackChatClient) GetCachedChannelType(teamID string, channel string) (ChannelType, error) {
	key := fmt.Sprintf("channel:%s:%s", teamID, channel)
	b, err := c.ecm.redisClient.Get(key).Bytes()
	if err != nil {
		return Unknown, nil
	}
	fmt.Printf("channel type before unmarshall is: %#v\n", b)
	var ret ChannelType
	err = ret.UnmarshalBinary(b)
	return ret, err
}

// SetCachedChannelType stores a channel type in redis for a given channel string
func (c *SlackChatClient) SetCachedChannelType(teamID string, channel string, cType *ChannelType) error {
	key := fmt.Sprintf("channel:%s:%s", teamID, channel)
	return c.ecm.redisClient.Set(key, cType, time.Minute*120).Err()
}

// SpawnPodCrier is intended to be called from a go routine. calls CryPods() at freq duration specified
func (c *SlackChatClient) SpawnPodCrier(freq time.Duration) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()
	c.CryPods(time.Now())
	for tick := range ticker.C {
		if c.ShuttingDown {
			return
		}
		err := c.CryPods(tick)
		if err != nil {
			c.log.Errorf("Failed to announce pods: %s", err)
		}
	}
}

// SpawnTeamsCrier is intended to be called from a go routine. calls CryTeams() at freq duration specified
func (c *SlackChatClient) SpawnTeamsCrier(freq time.Duration) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()
	c.CryTeams(time.Now())
	for tick := range ticker.C {
		if c.ShuttingDown {
			return
		}
		err := c.CryTeams(tick)
		if err != nil {
			c.log.Errorf("Failed to announce teams: %s", err)
		}
	}
}

// CryPods announces this pods identity to redis
func (c *SlackChatClient) CryPods(tick time.Time) error {
	err := c.ecm.redisClient.HSet("pods", c.config.podName, tick.Format(timeFormat)).Err()
	pods, _ := c.GetHashKeys("pods")
	c.log.Debugf("Just cried pod '%v'. Current HSet value: %v", c.config.podName, pods)
	return err
}

// CryTeams announces all teams to redis
func (c *SlackChatClient) CryTeams(tick time.Time) error {
	teams, err := c.GetAllTeams(context.Background(), c.config.slackAppID)
	c.log.Debugf("Crying teams: %+v", teams)
	if err != nil {
		return fmt.Errorf("could not get teams from datastore: %s", err)
	}
	for teamID := range teams {
		go func(teamID string, tick time.Time) {
			err := c.ecm.redisClient.HSet("teams", teamID, tick.Format(timeFormat)).Err()
			if err != nil {
				c.log.Errorf("Could not cry team (%s): %s", teamID, err)
			}
		}(teamID, time.Now())
	}
	return nil
}

// SpawnReaper is intended to be called from a goroutine. calls reap() at the specified frequency and deletes objects older than the specified duration
func (c *SlackChatClient) SpawnReaper(key string, freq time.Duration, age time.Duration) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()
	for range ticker.C {
		if c.ShuttingDown {
			return
		}
		err := c.reap(key, freq, age)
		if err != nil {
			c.log.Errorf("Error Reaping %s: %s", key, err)
		}
	}
}

func (c *SlackChatClient) reap(key string, freq time.Duration, age time.Duration) error {
	hashMap, err := c.ecm.redisClient.HGetAll(key).Result()
	if err != nil {
		return fmt.Errorf("could not get hash for key '%s' for reap: %s", key, err)
	}
	for k, v := range hashMap {
		c.log.Debugf("k: %v v: %v\n", k, v)
		lastCheckin, err := time.Parse(timeFormat, v)
		if err != nil {
			c.log.Criticalf("Error parsing time. Deleting offending entry(%s): %v\n", k, err)
			c.ecm.redisClient.HDel(key, k)
			continue
		}
		if time.Since(lastCheckin) >= age {
			c.log.Debugf("Reaping %s: %s. They were %.3f seconds old", key, k, age.Seconds())
			c.ecm.redisClient.HDel(key, k).Err()
		}
	}
	return nil
}

// GetHashKeys returns a slice of strings for a given redis hash key
func (c *SlackChatClient) GetHashKeys(key string) ([]string, error) {
	return c.ecm.redisClient.HKeys(key).Result()
}

// AssignTeamToPod records teams assignment to pods
func (c *SlackChatClient) AssignTeamToPod(teamID string, podName string, expirey time.Duration) error {
	key := fmt.Sprintf("team-assignment:%s", teamID)
	return c.ecm.redisClient.Set(key, podName, expirey).Err()
}

// GetTeamsAssignedtoPod assigns teams to pods
func (c *SlackChatClient) GetTeamsAssignedtoPod() ([]string, error) {
	teams, err := c.GetHashKeys("teams")
	if err != nil {
		return nil, err
	}
	pods, err := c.GetHashKeys("pods")
	if err != nil {
		return nil, err
	}
	c.log.Debugf("Assigning Teams to Pods: Teams: %v, Pods: %v, Me: %v.", teams, pods, c.config.podName)
	ring := hashring.New(pods)
	var assignedTeams []string
	for _, teamID := range teams {
		pod, ok := ring.GetNode(teamID)
		if !ok {
			c.log.Criticalf("Failed to hash pod(%s) for team(%s) ", pod, teamID)
		}
		if pod == c.config.podName {
			assignedTeams = append(assignedTeams, teamID)
		}
	}
	return assignedTeams, nil
}

// DeletePod deletes the current pod from the "pods" hashset
func (c *SlackChatClient) DeletePod() error {
	return c.ecm.redisClient.HDel("pods", c.config.podName).Err()
}
