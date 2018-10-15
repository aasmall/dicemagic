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

func dialDiceServer() bool {
	if initd == false {
		logger.Println("dialDiceServer")
		grpc.EnableTracing = true
		// Set up a connection to the dice-server.
		c, err := grpc.Dial(diceServerPort,
			grpc.WithInsecure(),
			grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
		if err != nil {
			log.Panicf("did not connect to dice-server(%s): %v", diceServerPort, err)
		}
		conn = c
	}
	return true
}

func QueryStringRollHandler(w http.ResponseWriter, r *http.Request) {
	initd = dialDiceServer()
	rollerClient := pb.NewRollerClient(conn)
	timeOutCtx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	cmd := r.URL.Query().Get("cmd")
	prob, _ := strconv.ParseBool(r.URL.Query().Get("p"))
	diceServerResponse, err := rollerClient.Roll(timeOutCtx, &pb.RollRequest{Cmd: cmd, Probabilities: prob})
	if err != nil {
		log.Println("could not roll: %v", err)
		return
	}

	fmt.Fprintf(w, "%+v", diceServerResponse)
}
func sortProbMap(m map[int64]float64) []int64 {
	var keys []int64
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
