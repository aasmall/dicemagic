module github.com/aasmall/dicemagic

go 1.14

//replace github.com/aasmall/gocui => ../gocui

//replace github.com/aasmall/asciigraph => ../asciigraph

require (
	cloud.google.com/go/datastore v1.5.0
	cloud.google.com/go/logging v1.3.0
	github.com/aasmall/asciigraph v0.4.2
	github.com/aasmall/gocui v0.4.0
	github.com/aasmall/word2number v0.0.0-20180508050052-3e177d961031
	github.com/davecgh/go-spew v1.1.1
	github.com/gizak/termui/v3 v3.1.0
	github.com/go-redis/redis/v7 v7.4.0
	github.com/gobuffalo/envy v1.9.0
	github.com/gohugoio/hugo v0.82.0 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.0 // indirect
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/serialx/hashring v0.0.0-20200727003509-22c0c7ab6b1b
	github.com/slack-go/slack v0.8.2
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.1
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4
	gonum.org/v1/gonum v0.9.1
	google.golang.org/api v0.44.0
	google.golang.org/grpc v1.37.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/ini.v1 v1.62.0 // indirect
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v0.20.5
)
