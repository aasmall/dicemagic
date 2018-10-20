package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
)

func upsertSlackInstallInstance(ctx context.Context, env *env, d *SlackInstallInstanceDoc) (*datastore.Key, error) {
	var err error
	var k *datastore.Key
	_, err = env.datastoreClient.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var docs []SlackInstallInstanceDoc

		q := datastore.NewQuery("SlackInstallInstance").Filter("TeamID =", d.TeamID).Filter("UserID =", d.UserID)
		_, err = env.datastoreClient.GetAll(ctx, q, &docs)
		if err != nil {
			return err
		}

		if docsLen := len(docs); docsLen > 1 {
			return fmt.Errorf("Multiple Matching SlackInstallInstance: %+v", docs)
		} else if docsLen == 1 {
			k, err = env.datastoreClient.Put(ctx, docs[0].Key, d)
			if err != nil {
				return err
			}
			return nil
		} else {
			k, err = env.datastoreClient.Put(ctx, datastore.IncompleteKey("SlackInstallInstance", nil), d)
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

func getFirstSlackInstallInstance(env env, ctx context.Context, teamID string, userID string) (*SlackInstallInstanceDoc, error) {
	var docs []SlackInstallInstanceDoc
	q := datastore.NewQuery("SlackInstallInstance").Filter("TeamID =", teamID).Filter("UserID =", userID).Limit(1)
	_, err := env.datastoreClient.GetAll(ctx, q, &docs)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no SlackInstallInstance found")
	}
	return &docs[0], nil
}
