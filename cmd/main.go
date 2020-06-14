package main

import (
	"context"
	"os"
	"time"

	"github.com/aasmall/dicemagic/lib/dicelang"
	"github.com/davecgh/go-spew/spew"
	"google.golang.org/grpc"
)

func main() {
	diceMagicGRPCClient, err := grpc.Dial(os.Args[1], grpc.WithInsecure())
	if err != nil {
		spew.Printf("did not connect to dice-server: %v", err)
		return
	}

	rollResponse, err := Roll(diceMagicGRPCClient, os.Args[2])
	println(spew.Sprint(rollResponse))
	println(spew.Sprint(err))
}

// Roll calls supplied grpc client with a freeform text command and returns a dice roll
func Roll(client *grpc.ClientConn, cmd string) (*dicelang.RollResponse, error) {
	rollerClient := dicelang.NewRollerClient(client)
	timeOutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request := &dicelang.RollRequest{
		Cmd: cmd,
	}
	return rollerClient.Roll(timeOutCtx, request)
}
