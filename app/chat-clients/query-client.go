package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"go.opencensus.io/trace"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"go.opencensus.io/plugin/ocgrpc"

	"log"

	pb "github.com/aasmall/dicemagic/app/proto"
)

func QueryStringRollHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "QueryStringRollHandler")
	defer span.End()
	grpc.EnableTracing = true
	// Set up a connection to the dice-server.
	conn, err := grpc.Dial(serverAddress, grpc.WithInsecure(), grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		log.Panicf("did not connect to dice-server(%s): %v", serverAddress, err)
	}
	defer conn.Close()
	client := pb.NewRollerClient(conn)
	timeOutCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	cmd := r.URL.Query().Get("cmd")
	prob, _ := strconv.ParseBool(r.URL.Query().Get("p"))
	diceServerResponse, err := client.Roll(timeOutCtx, &pb.RollRequest{Cmd: cmd, Probabilities: prob})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
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
