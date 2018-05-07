package internal

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/taskqueue"
)

//CommandType is an enum of supported commands
type CommandType int

const (
	//DiceRoll represents a command to roll dice.
	DiceRoll CommandType = iota
	//Decision represents a command to make a decision
	Decision
)

//go:generate stringer -type=CommandType

//Command represents a user command that can be saved,
//retrieved, and parsed from a string.
type Command interface {
	Save(ctx context.Context, key string) error
	Get(ctx context.Context, key string) error
	FromString(inputString ...string) error
	String() string
}

//compiletime check that RollCommand implements the
//comand interface
var _ Command = &RollCommand{}

//RollCommand represents multiple RollExpressions
type RollCommand struct {
	RollExpresions []RollExpression
}

func (r *RollCommand) String() string {
	var buffer bytes.Buffer
	for i, e := range r.RollExpresions {
		if i == len(r.RollExpresions) {
			buffer.WriteString(e.String())
		} else {
			buffer.WriteString(e.String())
			buffer.WriteString("and ")
		}
	}
	return buffer.String()
}

//FromString parses input strings and returns a constructed RollCommand
//by calling the perser
func (r *RollCommand) FromString(inputString ...string) error {
	for _, s := range inputString {
		expression, err := NewParser(strings.NewReader(s)).Parse()
		if err != nil {
			return err
		}
		r.RollExpresions = append(r.RollExpresions, *expression)
	}
	return nil
}

//Get retrieves a RollCommand from the DB and populates the struct
//using memcache if possible
func (r *RollCommand) Get(ctx context.Context, key string) error {
	var c RollCommand
	_, err := memcache.Gob.Get(ctx, key, &c)
	if err != nil {
		log.Infof(ctx, "cache miss (%s): %s", key, err)
		db, err := ConfigureDatastoreDB(ctx)
		if err != nil {
			return err
		}
		command, err := db.GetRoll(ctx, key)
		if err != nil {
			return err
		}
		r.RollExpresions = command.RollExpresions
		return nil
	}
	r.RollExpresions = c.RollExpresions
	return nil
}

//Save persists a RollCommand to the DB
//using memcache if possible
func (r *RollCommand) Save(ctx context.Context, key string) error {
	item := &memcache.Item{
		Key:    key,
		Object: r}
	err := memcache.Gob.Set(ctx, item)
	if err != nil {
		log.Infof(ctx, "Failed to save RollCommand to memcache: %s", err)
		// take perf hit, save to db now
		db, err := ConfigureDatastoreDB(ctx)
		if err != nil {
			log.Criticalf(ctx, err.Error())
			return err
		}
		err = db.UpsertRoll(ctx, key, r)
		if err != nil {
			log.Criticalf(ctx, err.Error())
			return err
		}
		return nil
	}
	//issue task to read from memcache and persist
	v := url.Values{"key": []string{key}}
	t := taskqueue.NewPOSTTask("/savecommand", v)
	log.Debugf(ctx, "Issuing task to save command at key: %v", v)
	tx, err := taskqueue.Add(ctx, t, "default")
	if err != nil {
		log.Criticalf(ctx, "Task Add Error %s\n%v", err, tx)
		return err
	}
	return nil
}
