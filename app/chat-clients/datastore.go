package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aasmall/dicemagic/app/dicelang/errors"

	"cloud.google.com/go/datastore"
)

type SlackTeam struct {
	Key      *datastore.Key `datastore:"__key__"`
	TeamID   string
	TeamName string
	Pod      string
}
type SlackInstallInstanceStatusDoc struct {
	Key      *datastore.Key `datastore:"__key__"`
	Pod      string
	LastSeen time.Time
	Open     bool
}

// func AssignTeamToPod(ctx context.Context, env *env, teamKey *datastore.Key, pod string) error {
// 	var err error
// 	_, err = env.datastoreClient.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
// 		team := &SlackTeam{}
// 		err := env.datastoreClient.Get(ctx, teamKey, team)
// 		if err != nil {
// 			env.log.Errorf("could not get team to assign to pod: %s", err)
// 			return err
// 		}
// 		team.Pod = pod
// 		_, err = env.datastoreClient.Put(ctx, teamKey, team)
// 		if err != nil {
// 			env.log.Errorf("could not put team to assign to pod: %s", err)
// 			return err
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

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
func getAllTeams(ctx context.Context, env *env) (map[string]*datastore.Key, error) {
	q := datastore.NewQuery("SlackTeam")
	var teams []SlackTeam
	_, err := env.datastoreClient.GetAll(ctx, q, &teams)
	if err != nil {
		return nil, err
	}
	retMap := make(map[string]*datastore.Key)
	for _, team := range teams {
		retMap[team.TeamID] = team.Key
	}
	return retMap, nil
}
func getAllTeamsForPod(ctx context.Context, env *env) (map[string]*datastore.Key, error) {
	q := datastore.NewQuery("SlackTeam").Filter("Pod =", env.config.podName)
	var teams []SlackTeam
	_, err := env.datastoreClient.GetAll(ctx, q, &teams)
	if err != nil {
		return nil, err
	}
	retMap := make(map[string]*datastore.Key)
	for _, team := range teams {
		retMap[team.TeamID] = team.Key
	}
	// if env.isLocal() {
	// 	fmt.Printf("keys for pod(%s): %+v", env.config.podName, retMap)
	// }
	// env.log.Infof("keys for pod(%s): %+v", env.config.podName, retMap)
	return retMap, nil
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

// // func isSlackInstanceAssigned(ctx context.Context, env *env, instanceKey *datastore.Key)
// type tsSlackInstallInstanceDoc struct {
// 	doc *SlackInstallInstanceDoc
// 	mux sync.Mutex
// }

// func getAllSlackInstallInstancesForTeam(ctx context.Context, env *env, teamID string) (<-chan *SlackInstallInstanceDoc, <-chan error) {
// 	out := make(chan *SlackInstallInstanceDoc)
// 	outErr := make(chan error)
// 	go func() {
// 		q := datastore.NewQuery("SlackTeam").Filter("TeamID =", teamID).KeysOnly()
// 		teams, err := env.datastoreClient.GetAll(ctx, q, nil)
// 		for _, key := range teams {
// 			q := datastore.NewQuery("SlackInstallInstance").Ancestor(key)
// 			for t := env.datastoreClient.Run(ctx, q); ; {
// 				var doc SlackInstallInstanceDoc
// 				_, err := t.Next(&doc)
// 				if err == iterator.Done {
// 					break
// 				}
// 				if err != nil {
// 					outErr <- err
// 				}
// 				out <- &doc
// 			}
// 		}
// 		close(outErr)
// 		close(out)
// 	}()
// 	return out, outErr
// }

func getSlackInstallInstanceByKey(ctx context.Context, env *env, k *datastore.Key) (*SlackInstallInstanceDoc, error) {
	doc := &SlackInstallInstanceDoc{}
	err := env.datastoreClient.Get(ctx, k, doc)
	if err != nil {
		return &SlackInstallInstanceDoc{}, err
	}
	return doc, nil
}
func GetFirstSlackInstallInstanceByTeamID(ctx context.Context, env *env, teamID string) (*SlackInstallInstanceDoc, error) {
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
