package main

import (
	"log"
	"os"
	"time"

	"github.com/aasmall/dicemagic/lib/dicelang"
	"go.opencensus.io/plugin/ocgrpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	address    = "localhost:50051"
	defaultCmd = "roll 1d20"
)

var conn *grpc.ClientConn
var initd bool

func dialDiceServer() bool {
	if initd == false {
		log.Println("dialDiceServer")
		grpc.EnableTracing = true
		// Set up a connection to the dice-server.
		c, err := grpc.Dial(address,
			grpc.WithInsecure(),
			grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
		if err != nil {
			log.Panicf("did not connect to dice-server(%s): %v", address, err)
		}
		conn = c
	}
	return true
}

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := dicelang.NewRollerClient(conn)

	// Contact the server and print out its response.
	cmd := defaultCmd
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for index := 0; index < 1000; index++ {

		r, err := c.Roll(ctx, &dicelang.RollRequest{Cmd: cmd})
		if err != nil {
			log.Fatalf("could not greet: %v", err)
		}
		log.Printf("%d: Greeting: %+v", index, r)
	}
}
