package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"go.opencensus.io/plugin/ocgrpc"

	"log"

	pb "github.com/aasmall/dicemagic/app/proto"
)

var (
	conn  *grpc.ClientConn
	initd bool
)

func dialDiceServer(env *env) bool {
	if initd == false {
		grpc.EnableTracing = true
		// Set up a connection to the dice-server.
		c, err := grpc.Dial(env.config.diceServerPort,
			grpc.WithInsecure(),
			grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
		if err != nil {
			log.Fatalf("did not connect to dice-server(%s): %v", env.config.diceServerPort, err)
		}
		conn = c
	}
	return true
}

func QueryStringRollHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, _ := e.(*env)
	log := env.log.WithRequest(r)
	initd = dialDiceServer(env)
	log.Info("dial dice server")
	rollerClient := pb.NewRollerClient(conn)
	timeOutCtx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	cmd := r.URL.Query().Get("cmd")
	prob, _ := strconv.ParseBool(r.URL.Query().Get("p"))
	diceServerResponse, err := rollerClient.Roll(timeOutCtx, &pb.RollRequest{Cmd: cmd, Probabilities: prob})
	if err != nil {
		env.log.Errorf("Oops! %s", err)
		return err
	}

	fmt.Fprintf(w, "%+v", diceServerResponse)
	return nil
}
func sortProbMap(m map[int64]float64) []int64 {
	var keys []int64
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
