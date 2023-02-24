module github.com/aasmall/dicemagic/dice-server

go 1.12

require (
	cloud.google.com/go/logging v1.6.1
	cloud.google.com/go/monitoring v1.12.0 // indirect
	cloud.google.com/go/trace v1.8.0 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.7.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.9.1
	github.com/aasmall/dicemagic/internal/dicelang v0.1.0
	github.com/aasmall/dicemagic/internal/dicelang/errors v0.1.0
	github.com/aasmall/dicemagic/internal/logger v0.1.0
	github.com/aasmall/dicemagic/internal/proto v0.1.0
	github.com/blend/go-sdk v1.20220411.3 // indirect
	github.com/wcharczuk/go-chart v2.0.1+incompatible
	go.opencensus.io v0.24.0
	golang.org/x/net v0.7.0
	gonum.org/v1/gonum v0.12.0 // indirect
	google.golang.org/grpc v1.51.0
)

replace github.com/aasmall/dicemagic/internal/dicelang v0.1.0 => ../internal/dicelang

replace github.com/aasmall/dicemagic/internal/dicelang/errors v0.1.0 => ../internal/dicelang/errors

replace github.com/aasmall/dicemagic/internal/logger v0.1.0 => ../internal/logger

replace github.com/aasmall/dicemagic/internal/proto v0.1.0 => ../internal/proto
