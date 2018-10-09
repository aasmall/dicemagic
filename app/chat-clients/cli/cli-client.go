package main

import (
	"log"
	"os"
	"time"

	pb "github.com/aasmall/dicemagic/app/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	address    = "localhost:50051"
	defaultCmd = "roll 1d20"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewRollerClient(conn)

	// Contact the server and print out its response.
	cmd := defaultCmd
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for index := 0; index < 1000; index++ {

		r, err := c.Roll(ctx, &pb.RollRequest{Cmd: cmd})
		if err != nil {
			log.Fatalf("could not greet: %v", err)
		}
		log.Printf("%d: Greeting: %+v", index, r)
	}
}
