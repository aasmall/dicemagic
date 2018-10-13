//go:generate protoc -I ../proto --go_out=plugins=grpc:../proto ../proto/dicemagic.proto

package main

import (
	"fmt"
	"net"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"log"

	"github.com/aasmall/dicemagic/app/dice-server/dicelang"
	pb "github.com/aasmall/dicemagic/app/proto"
	"golang.org/x/net/context"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
)

const (
	port      = ":50051"
	projectID = "k8s-dice-magic"
)

type server struct{}

func (s *server) Roll(ctx context.Context, in *pb.RollRequest) (*pb.RollResponse, error) {
	ctx, span := trace.StartSpan(ctx, "Roll")
	defer span.End()
	var out pb.RollResponse

	ctx, parseSpan := trace.StartSpan(ctx, "Parse")
	var p *dicelang.Parser
	p = dicelang.NewParser(in.Cmd)
	parseSpan.End()

	ctx, getDiceSetSpan := trace.StartSpan(ctx, "GetDiceSet")
	_, root, err := p.Statements()
	if err != nil {
		fmt.Println(err.Error(), err.(dicelang.LexError).Col, err.(dicelang.LexError).Line)
		return &pb.RollResponse{}, err
	}

	_, ds, err := root.GetDiceSet()
	if err != nil {
		fmt.Println(err.Error(), err.(dicelang.LexError).Col, err.(dicelang.LexError).Line)
		return &pb.RollResponse{}, err
	}
	getDiceSetSpan.End()

	ctx, restructureSpan := trace.StartSpan(ctx, "RestructureDice")
	var outDice []*pb.Dice
	for _, d := range ds.Dice {

		var dice pb.Dice
		dice.Color = d.Color
		dice.Count = d.Count
		dice.DropHighest = d.DropHighest
		dice.DropLowest = d.DropLowest
		dice.Faces = d.Faces
		dice.Max = d.Max
		dice.Min = d.Min
		dice.Sides = d.Sides
		dice.Total = d.Total
		if in.Probabilities {
			dice.Probabilities = dicelang.DiceProbability(dice.Count, dice.Sides, dice.DropHighest, dice.DropLowest)
		}
		if in.Chart {
			dice.Chart = []byte{}
		}
		outDice = append(outDice, &dice)
	}
	restructureSpan.End()
	var outDiceSet pb.DiceSet
	outDiceSet.Dice = outDice
	outDiceSet.TotalsByColor = ds.TotalsByColor
	out.DiceSet = &outDiceSet
	out.Cmd = in.Cmd
	return &out, nil
}

func main() {

	// Stackdriver Trace exporter
	grpc.EnableTracing = true
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID: projectID,
	})

	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	pb.RegisterRollerServer(s, &server{})

	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}