package main

import (
	"time"

	"github.com/aasmall/dicemagic/lib/dicelang"
	"golang.org/x/net/context"
)

// RollOption type to apply RollOptions
type RollOption func(*RollOptions)

// RollOptions contains options when calling dice-server
type RollOptions struct {
	Chart       bool
	Probability bool
	Timeout     time.Duration
	Context     context.Context
}

// RollOptionWithChart asks dice-server to generate a chart
func RollOptionWithChart(withChart bool) RollOption {
	return func(o *RollOptions) {
		o.Chart = withChart
	}
}

// RollOptionWithProbability asks dice-server to return probability map
func RollOptionWithProbability(withProb bool) RollOption {
	return func(o *RollOptions) {
		o.Probability = withProb
	}
}

// RollOptionWithTimeout specify a timeout
func RollOptionWithTimeout(timeout time.Duration) RollOption {
	return func(o *RollOptions) {
		o.Timeout = timeout
	}
}

// RollOptionWithContext specify a context
func RollOptionWithContext(ctx context.Context) RollOption {
	return func(o *RollOptions) {
		o.Context = ctx
	}
}

// Roll calls supplied grpc client with a freeform text command and returns a dice roll
func Roll(client dicelang.RollerClient, cmd string, options ...RollOption) (*dicelang.RollResponse, error) {
	opts := RollOptions{
		Chart:       false,
		Probability: false,
		Timeout:     time.Second,
		Context:     context.Background(),
	}
	for _, o := range options {
		o(&opts)
	}
	timeOutCtx, cancel := context.WithTimeout(opts.Context, opts.Timeout)
	defer cancel()
	request := &dicelang.RollRequest{
		Cmd:           cmd,
		Probabilities: opts.Probability,
		Chart:         opts.Chart,
	}
	return client.Roll(timeOutCtx, request)
}
