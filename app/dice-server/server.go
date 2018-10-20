//go:generate protoc -I ../proto --go_out=plugins=grpc:../proto ../proto/dicemagic.proto

package main

import (
	"net"
	"os"
	"reflect"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"log"

	"github.com/aasmall/dicemagic/app/dicelang"
	"github.com/aasmall/dicemagic/app/dicelang/errors"
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
	log.Println("dice-server up.")
	if err := s.Serve(lis); err != nil {
		env.log.Criticalf("failed to serve: %v", err)
		return
	}
}

func (s *server) handleExposedErrors(e error, response *pb.RollResponse) error {
	response.Error = &pb.RollError{}
	response.Ok = false
	switch e := e.(type) {
	case *errors.DicelangError:
		log.Println("DiceLangError")
		response.Error.Code = e.Code
		switch response.Error.Code {
		case errors.InvalidAST:
			response.Error.Msg = "The AST that resulted from your command was invalid."
		case errors.InvalidCommand:
			response.Error.Msg = "Your command could not be parsed."
		case errors.Friendly:
			response.Error.Msg = e.Error()
		case errors.Unexpected:
		default:
			panic("Unexpected error can't surface.")
		}
	default:
		response.Error.Code = errors.Unexpected
		response.Error.Msg = "An unexpected Error has occured. Please try again later"
		s.env.log.Criticalf("An unhandled error occured: %+v", e)
		log.Printf("%+v: An unhandled error occured: %+v", reflect.TypeOf(e).Elem(), e)
		return e
	}
	return nil
}

func astToPbDiceSets(p bool, c bool, ro bool, tree *dicelang.AST) ([]*pb.DiceSet, error) {

	if ro {
		total, ds, err := tree.GetDiceSet()
		if err != nil {
			return []*pb.DiceSet{}, err
		}
		restring, err := tree.String()
		if err != nil {
			return []*pb.DiceSet{}, err
		}
		pbDiceSet := &pb.DiceSet{
			Dice:          diceToPbDice(p, c, ds.Dice...),
			TotalsByColor: ds.TotalsByColor,
			Total:         int64(total),
			ReString:      restring,
		}
		return append([]*pb.DiceSet{}, pbDiceSet), nil
	}

	var outDiceSets []*pb.DiceSet
	if tree == nil {
		return nil, errors.NewDicelangError("No dice sets resulted from that command", errors.InvalidCommand, nil)
	}
	for _, child := range tree.Children {
		total, ds, err := child.GetDiceSet()
		if err != nil {
			return []*pb.DiceSet{}, err
		}
		restring, err := tree.String()
		if err != nil {
			return []*pb.DiceSet{}, err
		}
		outDiceSets = append(outDiceSets,
			&pb.DiceSet{
				Dice:          diceToPbDice(p, c, ds.Dice...),
				TotalsByColor: ds.TotalsByColor,
				Total:         int64(total),
				ReString:      restring,
			})
	}

	return outDiceSets, nil
}

func diceToPbDice(p bool, c bool, dice ...dicelang.Dice) []*pb.Dice {
	var outDice []*pb.Dice
	for _, d := range dice {
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
		if p {
			dice.Probabilities = dicelang.DiceProbability(dice.Count, dice.Sides, dice.DropHighest, dice.DropLowest)
		}
		if c {
			dice.Chart = []byte{}
		}
		outDice = append(outDice, &dice)
	}

	return outDice
}

func (s *server) Roll(ctx context.Context, in *pb.RollRequest) (*pb.RollResponse, error) {
	ctx, span := trace.StartSpan(ctx, "Roll")
	defer span.End()
	out := pb.RollResponse{Ok: true}

	ctx, parseSpan := trace.StartSpan(ctx, "Parse")
	var p *dicelang.Parser
	p = dicelang.NewParser(in.Cmd)
	tree, err := p.Statements()
	parseSpan.End()

	diceSets, err := astToPbDiceSets(in.Probabilities, in.Chart, in.RootOnly, tree)
	if err != nil {
		return &out, s.handleExposedErrors(err, &out)
	}

	if len(diceSets) <= 1 {
		out.DiceSet = diceSets[0]
	} else {
		for _, ds := range diceSets {
			out.DiceSets = append(out.DiceSets, ds)
		}
	}

	out.Cmd = in.Cmd
	return &out, nil
}
