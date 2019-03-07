module github.com/aasmall/dicemagic/dice-server

go 1.12

require (
	cloud.google.com/go v0.36.0
	contrib.go.opencensus.io/exporter/stackdriver v0.9.1
	github.com/aasmall/dicemagic/internal/dicelang v0.1.0
	github.com/aasmall/dicemagic/internal/dicelang/errors v0.1.0
	github.com/aasmall/dicemagic/internal/logger v0.1.0
	github.com/aasmall/dicemagic/internal/proto v0.1.0
	github.com/census-instrumentation/opencensus-proto v0.1.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/wcharczuk/go-chart v2.0.1+incompatible
	go.opencensus.io v0.19.1
	golang.org/x/image v0.0.0-20190227222117-0694c2d4d067 // indirect
	golang.org/x/net v0.0.0-20190301231341-16b79f2e4e95
	google.golang.org/grpc v1.19.0
)

replace github.com/aasmall/dicemagic/internal/dicelang v0.1.0 => ../internal/dicelang

replace github.com/aasmall/dicemagic/internal/dicelang/errors v0.1.0 => ../internal/dicelang/errors

replace github.com/aasmall/dicemagic/internal/logger v0.1.0 => ../internal/logger

replace github.com/aasmall/dicemagic/internal/proto v0.1.0 => ../internal/proto
