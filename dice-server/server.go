//go:generate protoc -I ../proto --go_out=plugins=grpc:../proto ../proto/dicemagic.proto

package main

import (
	"net"
	"sort"

	"cloud.google.com/go/logging"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/aasmall/dicemagic/internal/dicelang"
	"github.com/aasmall/dicemagic/internal/dicelang/errors"
	log "github.com/aasmall/dicemagic/internal/logger"
	pb "github.com/aasmall/dicemagic/internal/proto"
	"golang.org/x/net/context"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
)

type env struct {
	log    *log.Logger
	config *envConfig
}

type envConfig struct {
	projectID        string
	logName          string
	serverPort       string
	debug            bool
	local            bool
	podName          string
	traceProbability float64
}
type server struct {
	env *env
}

func newServer(e *env) *server {
	s := &server{env: e}
	return s
}

func main() {
	configReader := new(envReader)

	config := &envConfig{
		projectID:        configReader.getEnv("PROJECT_ID"),
		logName:          configReader.getEnv("LOG_NAME"),
		serverPort:       configReader.getEnv("SERVER_PORT"),
		debug:            configReader.getEnvBoolOpt("DEBUG"),
		local:            configReader.getEnvBoolOpt("LOCAL"),
		traceProbability: configReader.getEnvFloat("TRACE_PROBABILITY"),
		podName:          configReader.getEnv("POD_NAME"),
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
	env.log = log.New(
		env.config.projectID,
		log.WithDefaultSeverity(logging.Error),
		log.WithDebug(env.config.debug),
		log.WithLocal(env.config.local),
		log.WithLogName(env.config.logName),
		log.WithPrefix(env.config.podName+": "),
	)
	env.log.Debug("Logger up and running!")
	defer log.Println("Shutting down logger.")
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
	log := s.env.log
	response.Error = &pb.RollError{}
	response.Ok = false
	switch e := e.(type) {
	case *errors.DicelangError:
		log.Debugf("DiceLangError: %+v", e)
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
		return e
	}
	return nil
}

func (s *server) astToPbDiceSets(p bool, c bool, ro bool, tree *dicelang.AST) (*pb.DiceSet, []*pb.DiceSet, error) {
	log := s.env.log
	var fTotal float64
	if tree == nil {
		return nil, nil, errors.NewDicelangError("No dice sets resulted from that command", errors.InvalidCommand, nil)
	}
	total, ds, err := tree.GetDiceSet()
	if err != nil {
		return nil, nil, err
	}
	restring, err := tree.String()
	if err != nil {
		return nil, nil, err
	}
	pbDiceSet := &pb.DiceSet{
		Dice:          diceToPbDice(p, c, ds.Dice...),
		TotalsByColor: ds.TotalsByColor,
		Total:         int64(total),
		ReString:      restring,
	}
	if ro {
		return pbDiceSet, []*pb.DiceSet{}, nil
	}

	var outDiceSets []*pb.DiceSet
	for _, child := range tree.Children {
		log.Debugf("child: %+v", child)
		if child.Value == "REP" {
			var sortabldDiceSets []*pb.DiceSet
			reps, _, _ := child.Children[1].GetDiceSet()
			for index := 0; index < int(reps); index++ {
				total, ds, err := child.Children[0].GetDiceSet()
				fTotal += total
				if err != nil {
					return nil, nil, err
				}
				restring, err := child.Children[0].String()
				if err != nil {
					return nil, nil, err
				}
				sortabldDiceSets = append(sortabldDiceSets,
					&pb.DiceSet{
						Dice:          diceToPbDice(p, c, ds.Dice...),
						TotalsByColor: ds.TotalsByColor,
						Total:         int64(total),
						ReString:      restring,
					})
			}
			sort.Slice(sortabldDiceSets, func(i, j int) bool {
				return sortabldDiceSets[i].Total < sortabldDiceSets[j].Total
			})
			outDiceSets = append(outDiceSets, sortabldDiceSets...)
		} else {
			total, ds, err := child.GetDiceSet()
			fTotal += total
			if err != nil {
				return nil, nil, err
			}
			restring, err := child.String()
			if err != nil {
				return nil, nil, err
			}
			outDiceSets = append(outDiceSets,
				&pb.DiceSet{
					Dice:          diceToPbDice(p, c, ds.Dice...),
					TotalsByColor: ds.TotalsByColor,
					Total:         int64(total),
					ReString:      restring,
				})
		}
	}
	pbDiceSet.Total = int64(fTotal)
	return pbDiceSet, outDiceSets, nil
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
	log := s.env.log
	ctx, span := trace.StartSpan(ctx, "Roll")
	defer span.End()
	out := pb.RollResponse{Ok: true}

	ctx, parseSpan := trace.StartSpan(ctx, "Parse")
	var p *dicelang.Parser
	if in.Cmd == "" {
		return &out, s.handleExposedErrors(errors.NewDicelangError("zero length command is invalid", errors.InvalidCommand, nil), &out)
	}
	p = dicelang.NewParser(in.Cmd)
	log.Debugf("Rolling cmd on server: %s", in.Cmd)
	tree, err := p.Statements()
	parseSpan.End()

	ctx, dsSpan := trace.StartSpan(ctx, "AST to Diceset")
	defer dsSpan.End()
	diceSet, diceSets, err := s.astToPbDiceSets(in.Probabilities, in.Chart, in.RootOnly, tree)
	if err != nil {
		return &out, s.handleExposedErrors(err, &out)
	}
	out.DiceSet = diceSet
	for _, ds := range diceSets {
		out.DiceSets = append(out.DiceSets, ds)
	}
	out.Cmd = in.Cmd
	log.Debugf("roll response from server: %+v", out)
	return &out, nil
}
