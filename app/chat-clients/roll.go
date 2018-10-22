package main

import (
	"time"

	"google.golang.org/grpc"

	pb "github.com/aasmall/dicemagic/app/proto"
	"golang.org/x/net/context"
)

type RollOption func(*RollOptions)
type RollOptions struct {
	Chart       bool
	Probability bool
	Timeout     time.Duration
	Context     context.Context
}

func RollOptionWithChart(withChart bool) RollOption {
	return func(o *RollOptions) {
		o.Chart = withChart
	}
}
func RollOptionWithProbability(withProb bool) RollOption {
	return func(o *RollOptions) {
		o.Probability = withProb
	}
}
func RollOptionWithTimeout(timeout time.Duration) RollOption {
	return func(o *RollOptions) {
		o.Timeout = timeout
	}
}
func RollOptionWithContext(ctx context.Context) RollOption {
	return func(o *RollOptions) {
		o.Context = ctx
	}
}

// GetDiceRoll calls supplied grpc client with a freeform text command and returns a dice roll
func Roll(client *grpc.ClientConn, cmd string, options ...RollOption) (*pb.RollResponse, error) {
	opts := RollOptions{
		Chart:       false,
		Probability: false,
		Timeout:     time.Second,
		Context:     context.Background(),
	}
	for _, o := range options {
		o(&opts)
	}
	rollerClient := pb.NewRollerClient(client)
	timeOutCtx, cancel := context.WithTimeout(opts.Context, opts.Timeout)
	defer cancel()
	request := &pb.RollRequest{
		Cmd:           cmd,
		Probabilities: opts.Probability,
		Chart:         opts.Chart,
	}
	return rollerClient.Roll(timeOutCtx, request)
}
