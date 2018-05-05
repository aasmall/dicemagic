package main

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
	UpsertRoll(ctx context.Context, namespace string, rollHash string, roll *RollCommand) error
	GetRoll(ctx context.Context, namespace string, rollHash string) (*RollCommand, error)
	Close()
}
type Integration struct {
	ID int64
	OAuthApprovalResponse
}

type Persistedroll struct {
	RollCommand `datastore:",noindex"`
	lastUsed    time.Time
}

func configureDatastoreDB(ctx context.Context) (DiceMagicDatabase, error) {
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

func (db *datastoreDB) UpsertRoll(ctx context.Context, namespace string, rollHash string, roll *RollCommand) error {
	k := db.datastoreRollKey(namespace, rollHash)
	r := Persistedroll{RollCommand: *roll, lastUsed: time.Now()}
	if _, err := db.client.Put(ctx, k, &r); err != nil {
		return fmt.Errorf("datastoredb: could not update roll: %v", err)
	}
	return nil
}

func (db *datastoreDB) GetRoll(ctx context.Context, namespace string, rollHash string) (*RollCommand, error) {
	k := db.datastoreRollKey(namespace, rollHash)
	roll := Persistedroll{}
	if err := db.client.Get(ctx, k, &roll); err != nil {
		return nil, fmt.Errorf("datastoredb: could not get Roll: %v", err)
	}
	command := roll.RollCommand
	return &command, nil
}

func (db *datastoreDB) datastoreKey(id int64) *datastore.Key {
	return datastore.IDKey("Integration", id, nil)
}
func (db *datastoreDB) datastoreRollKey(namespace string, rollHash string) *datastore.Key {
	key := datastore.NameKey("Roll", rollHash, nil)
	key.Namespace = namespace
	return key
}

func (db *datastoreDB) GetIntegration(ctx context.Context, id int64) (*Integration, error) {
	k := db.datastoreKey(id)
	integration := &Integration{}
	if err := db.client.Get(ctx, k, integration); err != nil {
		return nil, fmt.Errorf("datastoredb: could not get Integration: %v", err)
	}
	integration.ID = id
	return integration, nil
}

func (db *datastoreDB) AddIntegration(ctx context.Context, b *Integration) (id int64, err error) {
	k := datastore.IncompleteKey("Integration", nil)
	b.OAuthApprovalResponse.AccessToken, _ = encrypt(ctx, b.OAuthApprovalResponse.AccessToken)
	b.OAuthApprovalResponse.Bot.BotAccessToken, _ = encrypt(ctx, b.OAuthApprovalResponse.Bot.BotAccessToken)

	//
	//Enforce 1 Integration per team
	integrations, err := db.ListIntegrationsByTeam(ctx, b.OAuthApprovalResponse.TeamID)
	if err != nil {
		return 0, fmt.Errorf("datastoredb: could not query for duplicate Integration: %v", err)
	}
	if len(integrations) > 0 {
		k.ID = integrations[0].ID
	}

	k, err = db.client.Put(ctx, k, b)
	if err != nil {
		return 0, fmt.Errorf("datastoredb: could not put Integration: %v", err)
	}
	return k.ID, nil
}

func (db *datastoreDB) DeleteIntegration(ctx context.Context, id int64) error {
	k := db.datastoreKey(id)
	if err := db.client.Delete(ctx, k); err != nil {
		return fmt.Errorf("datastoredb: could not delete Integration: %v", err)
	}
	return nil
}

func (db *datastoreDB) UpdateIntegration(ctx context.Context, b *Integration) error {

	k := db.datastoreKey(b.ID)
	if _, err := db.client.Put(ctx, k, b); err != nil {
		return fmt.Errorf("datastoredb: could not update Integration: %v", err)
	}
	return nil
}

func (db *datastoreDB) ListIntegrations(ctx context.Context) ([]*Integration, error) {
	integrations := make([]*Integration, 0)
	q := datastore.NewQuery("Integration")

	keys, err := db.client.GetAll(ctx, q, &integrations)

	if err != nil {
		return nil, fmt.Errorf("datastoredb: could not list Integrations: %v", err)
	}

	for i, k := range keys {
		integrations[i].ID = k.ID
	}

	return integrations, nil
}
func (db *datastoreDB) ListIntegrationsByTeam(ctx context.Context, teamID string) ([]*Integration, error) {
	if teamID == "" {
		return db.ListIntegrations(ctx)
	}

	integrations := make([]*Integration, 0)
	q := datastore.NewQuery("Integration").
		Filter("TeamID =", teamID)

	keys, err := db.client.GetAll(ctx, q, &integrations)
	if err != nil {
		return nil, fmt.Errorf("datastoredb: could not list Integrations: %v", err)
	}

	for i, k := range keys {
		integrations[i].ID = k.ID
	}

	return integrations, nil
}
