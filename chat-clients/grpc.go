package main

import (
	"context"
	"time"

	"github.com/aasmall/dicemagic/lib/dicelang"
)

func (s grpcProxy) Roll(ctx context.Context, in *dicelang.RollRequest) (*dicelang.RollResponse, error) {
	timeOutCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	s.ecm.loggingClient.Debugf("recieved roll request: %+v", in)
	return s.ecm.rollerClient.Roll(timeOutCtx, in)
}
