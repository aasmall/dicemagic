package lib

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/appengine"
)

//compiletime check
var _ DiceMagicDatabase = &datastoreDB{}

type datastoreDB struct {
	client *datastore.Client
}

type DiceMagicDatabase interface {
	UpsertRoll(ctx context.Context, rollHash string, roll *RollCommand) error
	GetRoll(ctx context.Context, rollHash string) (*RollCommand, error)
	Close()
}

type Persistedroll struct {
	RollCommand `datastore:",noindex"`
	lastUsed    time.Time
}

func ConfigureDatastoreDB(ctx context.Context) (DiceMagicDatabase, error) {
	projectID := appengine.AppID(ctx)
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return newDatastoreDB(ctx, client)
}
func newDatastoreDB(ctx context.Context, client *datastore.Client) (DiceMagicDatabase, error) {
	t, err := client.NewTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastoredb: could not connect: %v", err)
	}
	if err := t.Rollback(); err != nil {
		return nil, fmt.Errorf("datastoredb: could not connect: %v", err)
	}
	return &datastoreDB{
		client: client}, nil
}

func (db *datastoreDB) Close() {
	db.client.Close()
}

func (db *datastoreDB) UpsertRoll(ctx context.Context, rollHash string, roll *RollCommand) error {
	k := db.datastoreRollKey(rollHash)
	r := Persistedroll{RollCommand: *roll, lastUsed: time.Now()}
	if _, err := db.client.Put(ctx, k, &r); err != nil {
		return fmt.Errorf("datastoredb: could not update roll: %v", err)
	}
	return nil
}

func (db *datastoreDB) GetRoll(ctx context.Context, rollHash string) (*RollCommand, error) {
	k := db.datastoreRollKey(rollHash)
	roll := Persistedroll{}
	if err := db.client.Get(ctx, k, &roll); err != nil {
		return nil, fmt.Errorf("datastoredb: could not get Roll: %v", err)
	}
	command := roll.RollCommand
	return &command, nil
}

func (db *datastoreDB) datastoreRollKey(rollHash string) *datastore.Key {
	key := datastore.NameKey("Roll", rollHash, nil)
	return key
}
