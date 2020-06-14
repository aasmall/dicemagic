package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/datastore"
)

// SlackTeam represents the datastructure for a Slack Team
type SlackTeam struct {
	Key        *datastore.Key `datastore:"__key__"`
	SlackAppID string
	TeamID     string
	TeamName   string
}

// SlackInstallInstanceDoc represents the data structure where we store all the resulting data from installing the bot.
type SlackInstallInstanceDoc struct {
	Key            *datastore.Key `datastore:"__key__"`
	EncAccessToken string         `datastore:",noindex" json:"access_token"`
	Bot            struct {
		EncBotAccessToken string `datastore:",noindex" json:"bot_access_token"`
		BotUserID         string `datastore:",noindex" json:"bot_user_id"`
	} `datastore:",noindex" json:"bot"`
	Ok       bool   `datastore:",noindex" json:"ok"`
	Error    string `datastore:",noindex" json:"error"`
	Scope    string `datastore:",noindex" json:"scope"`
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name"`
	UserID   string `json:"user_id"`
}

func main() {
	ctx := context.Background()
	ds, err := datastore.NewClient(ctx, "dice-magic-minikube")
	if err != nil {
		log.Printf("could not configure Datastore Client: %s", err)
		return
	}
	log.Printf("ClientCreated: %+v", ds)
	go createTeam(ctx, ds)
	select {}
}
func createTeam(ctx context.Context, ds *datastore.Client) {
	team := &SlackTeam{SlackAppID: "MOCKAPPID", TeamID: "mockTeamID", TeamName: "Mockery"}
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		fmt.Printf("%v+\n", time.Now())
		log.Printf("Upserting team to mock datastore: %v", team)
		k, err := UpsertSlackTeam(ctx, *ds, team)
		if err != nil {
			log.Printf("Error Upserting: %v", err)
		}
		// install doc
		instalDoc := &SlackInstallInstanceDoc{TeamID: "mockTeamID", TeamName: "Mockery", Ok: true}
		instalDoc.Bot.EncBotAccessToken = "QfWqQhjOB73q0khqrghxb5ssl9rnWg=="
		instalDoc.Bot.BotUserID = "FUBAR"
		log.Printf("Upserting install doc to mock datastore: %v", instalDoc)
		_, err = UpsertSlackInstallInstance(ctx, *ds, instalDoc, k)
		if err != nil {
			log.Printf("Error Upserting: %v", err)
		}
		time.Sleep(time.Second * 10)
	}
}

// UpsertSlackTeam either creates a new SlackTeam object or updates an existing one if it matches provided TeamID and SlackAppID
func UpsertSlackTeam(ctx context.Context, ds datastore.Client, team *SlackTeam) (*datastore.Key, error) {
	var err error
	var k *datastore.Key

	q := datastore.NewQuery("SlackTeam").Filter("TeamID = ", team.TeamID).Filter("SlackAppID = ", team.SlackAppID).KeysOnly()
	keys, err := ds.GetAll(ctx, q, &[]SlackTeam{})
	log.Printf("keys: %v", keys)
	if err != nil {
		return k, err
	}
	if keysLen := len(keys); keysLen > 1 {
		// Delete duplicate entries, but for now error out
		log.Printf("found multiple team entries for TeamID: %s, AppID: %s", team.TeamID, team.SlackAppID)
		return k, err
	} else if keysLen == 1 {
		k, err = ds.Put(ctx, keys[0], team)
		log.Printf("Updating SlackTeam with: %+v", team)
		if err != nil {
			return k, err
		}
		return k, nil
	} else {
		k, err = ds.Put(ctx, datastore.IncompleteKey("SlackTeam", nil), team)
		log.Printf("Creating SlackTeam with: %+v", team)
		if err != nil {
			return k, err
		}
		log.Printf("successfully upserted team: %+v", k)
		return k, nil
	}
}

// UpsertSlackInstallInstance either creates a new SlackInstallInstanceDoc or updates an existing one if it matches the UserID and parent of an existing one.
func UpsertSlackInstallInstance(ctx context.Context, ds datastore.Client, d *SlackInstallInstanceDoc, parentKey *datastore.Key) (*datastore.Key, error) {
	var err error
	var k *datastore.Key
	_, err = ds.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery("SlackInstallInstance").Ancestor(parentKey).Filter("UserID =", d.UserID).KeysOnly()
		keys, err := ds.GetAll(ctx, q, &[]SlackInstallInstanceDoc{})
		if err != nil {
			return err
		}
		if keysLen := len(keys); keysLen > 1 {
			// Delete duplicate entries, but for now error out
			err := fmt.Errorf("found multiple install entries entries for parent: %v, UserID: %s", parentKey, d.UserID)
			log.Print(err.Error())
			return err
		} else if keysLen == 1 {
			// Update
			k, err = ds.Put(ctx, keys[0], d)
			if err != nil {
				return err
			}
			return nil
		} else {
			// Insert
			k, err = ds.Put(ctx, datastore.IncompleteKey("SlackInstallInstance", parentKey), d)
			if err != nil {
				return err
			}
			return nil
		}
	})

	if err != nil {
		return nil, err
	}

	return k, nil
}
