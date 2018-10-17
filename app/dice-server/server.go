//go:generate protoc -I ../proto --go_out=plugins=grpc:../proto ../proto/dicemagic.proto

package main

import (
	"net"
	"os"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"log"

	"github.com/aasmall/dicemagic/app/dicelang"
	"github.com/aasmall/dicemagic/app/logger"
	pb "github.com/aasmall/dicemagic/app/proto"
	"golang.org/x/net/context"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
)

type env struct {
	log    *logger.Logger
	config *envConfig
}

type envConfig struct {
	projectID  string
	logName    string
	serverPort string
}

type envReader struct {
	missingKeys []string
	errors      bool
}

type server struct {
	env *env
}

func newServer(e *env) *server {
	s := &server{env: e}
	return s
}

func (r *envReader) getEnv(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	r.errors = true
	r.missingKeys = append(r.missingKeys, key)
	return ""
}

func main() {
	configReader := new(envReader)

	config := &envConfig{
		projectID:  configReader.getEnv("PROJECT_ID"),
		logName:    configReader.getEnv("LOG_NAME"),
		serverPort: configReader.getEnv("SERVER_PORT"),
	}
	if configReader.errors {
		log.Fatalf("could not gather environment variables. Failed variables: %v", configReader.missingKeys)
	}
	env := &env{config: config}

	//Stackdriver Trace exporter
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID: env.config.projectID,
	})
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	trace.RegisterExporter(exporter)

	// Stackdriver Logger
	env.log = logger.NewLogger(context.Background(), env.config.projectID, env.config.logName)
	defer env.log.Info("Shutting down logger.")
	defer env.log.Close()

	lis, err := net.Listen("tcp", env.config.serverPort)
	if err != nil {
		env.log.Criticalf("failed to listen: %v", err)
		return
	}
	s := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	pb.RegisterRollerServer(s, newServer(env))

	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		env.log.Criticalf("failed to serve: %v", err)
		return
	}
}

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
		s.env.log.Error(err.Error())
		return &pb.RollResponse{}, err
	}

	total, ds, err := root.GetDiceSet()
	if err != nil {
		s.env.log.Error(err.Error())
		return &pb.RollResponse{}, err
	}
	getDiceSetSpan.End()

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
	var outDiceSet pb.DiceSet
	outDiceSet.Dice = outDice
	outDiceSet.TotalsByColor = ds.TotalsByColor
	outDiceSet.ReString = root.String()
	outDiceSet.Total = int64(total)
	out.DiceSet = &outDiceSet
	out.Cmd = in.Cmd
	return &out, nil
}
