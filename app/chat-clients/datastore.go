package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aasmall/dicemagic/app/dicelang/errors"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
)

func upsetSlackTeam(ctx context.Context, env *env, team *SlackTeam) (*datastore.Key, error) {
	var err error
	var k *datastore.Key

	_, err = env.datastoreClient.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery("SlackTeam").Filter("TeamID = ", team.TeamID).KeysOnly()
		keys, err := env.datastoreClient.GetAll(ctx, q, &[]SlackTeam{})
		if err != nil {
			return err
		}
		if keysLen := len(keys); keysLen > 1 {
			// Delete duplicate entries, but for now error out
			err := errors.New(fmt.Sprintf("found multiple team entries for TeamID: %s", team.TeamID))
			env.log.Critical(err.Error())
			return err
		} else if keysLen == 1 {
			k, err = env.datastoreClient.Put(ctx, keys[0], team)
			if err != nil {
				return err
			}
			return nil
		} else {
			k, err = env.datastoreClient.Put(ctx, datastore.IncompleteKey("SlackTeam", nil), team)
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

func upsertSlackInstallInstance(ctx context.Context, env *env, d *SlackInstallInstanceDoc, parentKey *datastore.Key) (*datastore.Key, error) {
	var err error
	var k *datastore.Key
	_, err = env.datastoreClient.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery("SlackInstallInstance").Ancestor(parentKey).Filter("UserID =", d.UserID).KeysOnly()
		keys, err := env.datastoreClient.GetAll(ctx, q, &[]SlackInstallInstanceDoc{})
		if err != nil {
			return err
		}
		if keysLen := len(keys); keysLen > 1 {
			// Delete duplicate entries, but for now error out
			err := errors.New(fmt.Sprintf("found multiple install entries entries for parent: %v, UserID: %s", parentKey, d.UserID))
			env.log.Critical(err.Error())
			return err
		} else if keysLen == 1 {
			// Update
			k, err = env.datastoreClient.Put(ctx, keys[0], d)
			if err != nil {
				return err
			}
			return nil
		} else {
			// Insert
			k, err = env.datastoreClient.Put(ctx, datastore.IncompleteKey("SlackInstallInstance", parentKey), d)
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

func updateSlackInstanceStatusLastSeen(ctx context.Context, env *env, pod string, parentKey *datastore.Key, open bool) error {
	_, err := env.datastoreClient.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		updatedDoc := &SlackInstallInstanceStatusDoc{Pod: pod, LastSeen: time.Now(), Open: open}

		q := datastore.NewQuery("SlackInstallInstanceStatus").Ancestor(parentKey).KeysOnly()
		keys, err := env.datastoreClient.GetAll(ctx, q, nil)
		if err != nil {
			return err
		}
		if keysLen := len(keys); keysLen > 1 {
			// Delete duplicate entries, but for now error out
			err := errors.New(fmt.Sprintf("found multiple statuses for parent: %v", parentKey))
			env.log.Critical(err.Error())
			return err
		} else if keysLen == 1 {
			_, err = env.datastoreClient.Put(ctx, keys[0], updatedDoc)
			if err != nil {
				return err
			}
			return nil
		} else {
			_, err = env.datastoreClient.Put(ctx, datastore.IncompleteKey("SlackInstallInstanceStatus", parentKey), updatedDoc)
			if err != nil {
				return err
			}
			return nil
		}
	})
	return err
}
func getAllTeamsKeys(ctx context.Context, env *env) (*[]*datastore.Key, error) {
	q := datastore.NewQuery("SlackTeam").KeysOnly()
	teamKeys, err := env.datastoreClient.GetAll(ctx, q, nil)
	if err != nil {
		return nil, err
	}
	return &teamKeys, nil
}

func getAllStatusForTeamID(ctx context.Context, env *env, teamID string) ([]SlackInstallInstanceStatusDoc, error) {
	var docs []SlackInstallInstanceStatusDoc
	q := datastore.NewQuery("SlackTeam").Filter("TeamID = ", teamID).KeysOnly()
	teamKeys, err := env.datastoreClient.GetAll(ctx, q, nil)
	if err != nil {
		return nil, err
	}
	if keysLen := len(teamKeys); keysLen > 1 || keysLen < 1 {
		// Delete duplicate entries, but for now error out
		err := errors.New(fmt.Sprintf("found multiple or zero teams for for TeamID: %s", teamID))
		env.log.Critical(err.Error())
		return nil, err
	}
	// 1 key found
	q = datastore.NewQuery("SlackInstallInstanceStatus").Ancestor(teamKeys[0])
	_, err = env.datastoreClient.GetAll(ctx, q, &docs)
	if err != nil {
		return nil, err
	}
	return docs, err
}

func getFirstSlackInstallInstance(ctx context.Context, env *env, teamID string, userID string) (*SlackInstallInstanceDoc, error) {
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

// func getFirstSlackInstallStatusByAncestor(ctx context.Context, env *env,   *datastore.Key) {
// 	q := datastore.NewQuery("SlackInstallInstanceStatus").Ancestor(instanceKey)
// }

// func isSlackInstanceAssigned(ctx context.Context, env *env, instanceKey *datastore.Key)
type tsSlackInstallInstanceDoc struct {
	doc *SlackInstallInstanceDoc
	mux sync.Mutex
}

func getAllSlackInstallInstances(ctx context.Context, env *env) (<-chan *tsSlackInstallInstanceDoc, <-chan error) {
	out := make(chan *tsSlackInstallInstanceDoc)
	outErr := make(chan error)
	teamKeys, err := getAllTeamsKeys(ctx, env)
	if err != nil {
		env.log.Criticalf("could not get team keys: %s", err)
		outErr <- err
		close(outErr)
		close(out)
		return out, outErr
	}
	var wg sync.WaitGroup
	go func() {
		for _, key := range *teamKeys {
			wg.Add(1)
			q := datastore.NewQuery("SlackInstallInstance").Ancestor(key)
			go func(q datastore.Query, key *datastore.Key) {
				wg.Add(1)
				defer wg.Done()
				for t := env.datastoreClient.Run(ctx, &q); ; {
					var doc SlackInstallInstanceDoc
					_, err := t.Next(&doc)
					if err == iterator.Done {
						wg.Done()
						break
					}
					if err != nil {
						outErr <- err
					}
					key := key
					sq := datastore.NewQuery("SlackInstallInstanceStatus").Ancestor(key)
					statusDocs := []SlackInstallInstanceStatusDoc{}
					_, err = env.datastoreClient.GetAll(ctx, sq, &statusDocs)
					if err != nil {
						outErr <- err
					} else if len(statusDocs) == 0 {
						out <- &tsSlackInstallInstanceDoc{doc: &doc}
					} else if !statusDocs[0].Open || time.Now().Sub(statusDocs[0].LastSeen).Seconds() >= 90 {
						out <- &tsSlackInstallInstanceDoc{doc: &doc}
					}
				}

			}(*q, key)
		}
		wg.Wait()
		close(outErr)
		close(out)
	}()
	return out, outErr
}

func getSlackInstallInstanceByKey(ctx context.Context, env *env, k *datastore.Key) (*SlackInstallInstanceDoc, error) {
	doc := &SlackInstallInstanceDoc{}
	err := env.datastoreClient.Get(ctx, k, doc)
	if err != nil {
		return &SlackInstallInstanceDoc{}, err
	}
	return doc, nil
}
func getFirstSlackInstallInstanceByTeamID(ctx context.Context, env *env, teamID string) (*SlackInstallInstanceDoc, error) {
	docs := []SlackInstallInstanceDoc{}
	q := datastore.NewQuery("SlackInstallInstance").Filter("TeamID = ", teamID).Limit(1)
	_, err := env.datastoreClient.GetAll(ctx, q, &docs)
	if err != nil {
		return &SlackInstallInstanceDoc{}, err
	}
	if len(docs) != 1 {
		return &SlackInstallInstanceDoc{}, errors.New("somehow ended up with more than one result")
	}
	return &docs[0], nil
}

func deleteSlackInstallInstance(ctx context.Context, env *env, key *datastore.Key) error {
	_, err := env.datastoreClient.RunInTransaction(ctx, func(tx *datastore.Transaction) error {

		err := env.datastoreClient.Delete(ctx, key)
		if err != nil {
			return err
		}
		// Delete team and all children if this was the last instance
		q := datastore.NewQuery("SlackInstallInstance").Ancestor(key.Parent).KeysOnly()
		allKeys, err := env.datastoreClient.GetAll(ctx, q, nil)
		if err != nil {
			return err
		}
		if len(allKeys) == 0 {
			err = deleteWithAllChildren(ctx, env, key.Parent)
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

func deleteWithAllChildren(ctx context.Context, env *env, key *datastore.Key) error {
	_, err := env.datastoreClient.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery("").Ancestor(key).KeysOnly()
		allKeys, err := env.datastoreClient.GetAll(ctx, q, nil)
		if err != nil {
			return err
		}
		err = env.datastoreClient.DeleteMulti(ctx, append(allKeys, key))
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
