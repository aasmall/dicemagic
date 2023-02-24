module github.com/aasmall/dicemagic/chat-clients

go 1.12

require (
	cloud.google.com/go/datastore v1.1.0
	cloud.google.com/go/logging v1.6.1
	contrib.go.opencensus.io/exporter/ocagent v0.7.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.9.1
	github.com/aasmall/dicemagic v0.0.0-20190306205428-6b9ac5ae3d91
	github.com/aasmall/dicemagic/internal/dicelang/errors v0.1.0
	github.com/aasmall/dicemagic/internal/handler v0.1.0
	github.com/aasmall/dicemagic/internal/logger v0.1.0
	github.com/aasmall/dicemagic/internal/proto v0.1.0
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/gorilla/mux v1.7.0
	github.com/lusis/go-slackbot v0.0.0-20210303200821-3c34a039d473 // indirect
	github.com/lusis/slack-test v0.0.0-20190426140909-c40012f20018 // indirect
	github.com/nlopes/slack v0.5.0
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.27.1 // indirect
	github.com/serialx/hashring v0.0.0-20180504054112-49a4782e9908
	github.com/smartystreets/goconvey v1.7.2 // indirect
	go.opencensus.io v0.24.0
	golang.org/x/net v0.7.0
	golang.org/x/oauth2 v0.0.0-20221014153046-6fdb5e3db783
	google.golang.org/api v0.103.0
	google.golang.org/grpc v1.50.1
)

replace github.com/aasmall/dicemagic/internal/dicelang v0.1.0 => ../internal/dicelang

replace github.com/aasmall/dicemagic/internal/dicelang/errors v0.1.0 => ../internal/dicelang/errors

replace github.com/aasmall/dicemagic/internal/handler v0.1.0 => ../internal/handler

replace github.com/aasmall/dicemagic/internal/logger v0.1.0 => ../internal/logger

replace github.com/aasmall/dicemagic/internal/proto v0.1.0 => ../internal/proto
