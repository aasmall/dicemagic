package main

import (
	"context"
	"fmt"

	"github.com/aasmall/dicemagic/app/dicelang/errors"

	"cloud.google.com/go/datastore"
)

// SlackTeam represents a Slack Team that has installed Dice Magic
type SlackTeam struct {
	Key      *datastore.Key `datastore:"__key__"`
	TeamID   string
	TeamName string
}

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

func (ds *SlackDatastoreClient) UpsetSlackTeam(ctx context.Context, team *SlackTeam) (*datastore.Key, error) {
	var err error
	var k *datastore.Key

	_, err = ds.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery("SlackTeam").Filter("TeamID = ", team.TeamID).KeysOnly()
		keys, err := ds.GetAll(ctx, q, &[]SlackTeam{})
		if err != nil {
			return err
		}
		if keysLen := len(keys); keysLen > 1 {
			// Delete duplicate entries, but for now error out
			err := errors.New(fmt.Sprintf("found multiple team entries for TeamID: %s", team.TeamID))
			ds.log.Critical(err.Error())
			return err
		} else if keysLen == 1 {
			k, err = ds.Put(ctx, keys[0], team)
			if err != nil {
				return err
			}
			return nil
		} else {
			k, err = ds.Put(ctx, datastore.IncompleteKey("SlackTeam", nil), team)
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

func (ds *SlackDatastoreClient) UpsertSlackInstallInstance(ctx context.Context, d *SlackInstallInstanceDoc, parentKey *datastore.Key) (*datastore.Key, error) {
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
			err := errors.New(fmt.Sprintf("found multiple install entries entries for parent: %v, UserID: %s", parentKey, d.UserID))
			ds.log.Critical(err.Error())
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

func (ds *SlackDatastoreClient) GetAllTeams(ctx context.Context) (map[string]*datastore.Key, error) {
	q := datastore.NewQuery("SlackTeam")
	var teams []SlackTeam
	_, err := ds.GetAll(ctx, q, &teams)
	if err != nil {
		return nil, err
	}
	retMap := make(map[string]*datastore.Key)
	for _, team := range teams {
		retMap[team.TeamID] = team.Key
	}
	return retMap, nil
}
func (ds *SlackDatastoreClient) GetFirstSlackInstallInstanceByTeamID(ctx context.Context, teamID string) (*SlackInstallInstanceDoc, error) {
	docs := []SlackInstallInstanceDoc{}
	q := datastore.NewQuery("SlackInstallInstance").Filter("TeamID = ", teamID).Limit(1)
	_, err := ds.GetAll(ctx, q, &docs)
	if err != nil {
		return &SlackInstallInstanceDoc{}, err
	}
	if len(docs) > 1 {
		return &SlackInstallInstanceDoc{}, errors.New("somehow ended up with more than one result")
	} else if len(docs) < 1 {
		return &SlackInstallInstanceDoc{}, errors.New("No Install instances found. should self-correct with next team-tick")
	}
	return &docs[0], nil
}

func (ds *SlackDatastoreClient) DeleteSlackInstallInstance(ctx context.Context, key *datastore.Key) error {
	_, err := ds.RunInTransaction(ctx, func(tx *datastore.Transaction) error {

		err := ds.Delete(ctx, key)
		if err != nil {
			return err
		}
		// Delete team and all children if this was the last instance
		q := datastore.NewQuery("SlackInstallInstance").Ancestor(key.Parent).KeysOnly()
		allKeys, err := ds.GetAll(ctx, q, nil)
		if err != nil {
			return err
		}
		if len(allKeys) == 0 {
			err = ds.DeleteWithAllChildren(ctx, key.Parent)
		}
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil

}

func (ds *SlackDatastoreClient) DeleteWithAllChildren(ctx context.Context, key *datastore.Key) error {
	_, err := ds.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery("").Ancestor(key).KeysOnly()
		allKeys, err := ds.GetAll(ctx, q, nil)
		if err != nil {
			return err
		}
		err = ds.DeleteMulti(ctx, append(allKeys, key))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
