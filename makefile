BUILD_DIR ?= ./out
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOVET=$(GOCMD) vet
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

PROTOCZIP=protoc-3.12.0-linux-x86_64.zip
PROTOCDLOC=https://github.com/protocolbuffers/protobuf/releases/download/v3.12.0/

LIBS=$(shell find "lib" -type f)
ALL=$(shell find -type f)
GENDEPS=$(shell grep -rl . -e '//go:generate' ; find -type f -name "*.proto" )



# DOCKERFILES=$(shell find "dockerfiles" -maxdepth 1 -type f | grep -E '.*\.(dockerfile)$$')
# NAMES=$(subst dockerfiles/,,$(subst .dockerfile,,$(DOCKERFILES)))

# REGISTRY?=registry.gitlab.com/aasmall/dicemagic/
# IMAGES=$(addprefix $(subst :,\:,$(REGISTRY))/,$(NAMES))

$(shell mkdir -p $(BUILD_DIR))

.PHONY: ci
ci: gen core letsencrypt out/bin/www

.PHONY: local
local: gen core out/bin/www mocks secrets bootstrapsecrets

.PHONY: gen
gen: $(GENDEPS)
	@mkdir -p ${HOME}/.local
	export PATH=${HOME}/.local/bin:${PATH}
	echo $$PATH
	curl -sLO $(PROTOCDLOC)$(PROTOCZIP)
	unzip -qo $(PROTOCZIP) -d ${HOME}/.local
	rm $(PROTOCZIP)
	go get github.com/golang/protobuf/protoc-gen-go
	go get golang.org/x/tools/cmd/stringer
	PATH=${HOME}/.local/bin:${PATH} go generate ./...

out/bin/chat-clients: $(shell find "chat-clients" -maxdepth 1 -type f | grep -E '.*\.(go|mod|sum)$$')
	go build -o $@ github.com/aasmall/dicemagic/chat-clients

out/bin/dice-server: $(shell find "dice-server" -maxdepth 1 -type f | grep -E '.*\.(go|mod|sum)$$')
	go build -o $@ github.com/aasmall/dicemagic/dice-server

out/bin/redis-cluster: $(shell find "redis" -maxdepth 1 -type f | grep -E '.*\.(go|mod|sum|sh|conf)$$')
	@mkdir -p out/include/redis-cluster
	go build -o $@ github.com/aasmall/dicemagic/redis
	cp redis/bootstrap-pod.sh redis/redis.conf out/include/redis-cluster

.PHONY: core
core: out/bin/chat-clients out/bin/dice-server out/bin/redis-cluster

out/bin/mocks/datastore: $(shell find "mocks/datastore" -maxdepth 1 -type f | grep -E '.*\.(go|mod|sum|sh)$$') config/minikube/secrets/google/k8s-dice-magic.json
	@mkdir -p out/include/mocks/datastore/
	go build -o $@ github.com/aasmall/dicemagic/mocks/datastore
	cp mocks/datastore/bootstrap-datastore.sh out/include/mocks/datastore/

out/bin/mocks/kms: $(shell find "mocks/kms" -maxdepth 1 -type f | grep -E '.*\.(go|mod|sum)$$')
	go build -o $@ github.com/aasmall/dicemagic/mocks/kms

out/bin/mocks/slack-client: $(shell find "mocks/slack-client" -maxdepth 1 -type f | grep -E '.*\.(go)$$')
	go build -o $@ github.com/aasmall/dicemagic/mocks/slack-client

out/bin/mocks/slack-server: $(shell find "mocks/slack-server" -maxdepth 1 -type f | grep -E '.*\.(go|mod|sum)$$')
	go build -o $@ github.com/aasmall/dicemagic/mocks/slack-server

.PHONY: mocks
mocks: out/bin/mocks/datastore out/bin/mocks/kms out/bin/mocks/slack-client out/bin/mocks/slack-server

.PHONY: letsencrypt
letsencrypt: $(shell find "letsencrypt" -maxdepth 1 -type f | grep -E '.*\.(json|sh)$$')
	@mkdir -p out/include/letsencrypt
	cp letsencrypt/deployment-patch-template.json letsencrypt/renewcerts.sh letsencrypt/secret-patch-template.json out/include/letsencrypt

.PHONY: www
www: out/bin/www
out/bin/www: $(shell find "www" -type f)
	@mkdir -p out/include/www/local out/include/www/dev out/include/www/prod
	go build -o out/bin github.com/aasmall/dicemagic/www
	go install github.com/gohugoio/hugo
	hugo -s www/ -d ../out/include/www/local --config config-local.yaml
	hugo -s www/ -d ../out/include/www/dev --config config-dev.yaml
	hugo -s www/ -d ../out/include/www/prod --config config.yaml

.PHONY: bootstrapsecrets
bootstrapsecrets: config/development/secrets/letsencrypt-certs/tls.key config/development/secrets/letsencrypt-certs/tls.crt

config/development/secrets/letsencrypt-certs/tls.key config/development/secrets/letsencrypt-certs/tls.crt: config/development/openssl-letsencrypt.cnf
	@mkdir -p config/development/secrets/letsencrypt-certs
	@openssl ecparam -name secp521r1 -genkey -out ./config/development/secrets/letsencrypt-certs/tls.key      
	@openssl req -new -x509 -key ./config/development/secrets/letsencrypt-certs/tls.key -out ./config/development/secrets/letsencrypt-certs/tls.crt -days 3652 -config ./config/development/openssl-letsencrypt.cnf

config/development/secrets/nginx-ingress/tls.key config/development/secrets/nginx-ingress/tls.crt: config/development/openssl-nginx.cnf
	@mkdir -p config/development/secrets/nginx-ingress
	@openssl ecparam -name secp521r1 -genkey -out ./config/development/secrets/nginx-ingress/tls.key
	@openssl req -new -x509 -key ./config/development/secrets/nginx-ingress/tls.key -out ./config/development/secrets/nginx-ingress/tls.crt -days 3652 -config ./config/development/openssl-nginx.cnf

config/minikube/secrets/letsencrypt-certs/tls.key config/minikube/secrets/letsencrypt-certs/tls.crt: config/minikube/openssl-letsencrypt.cnf
	@mkdir -p config/minikube/secrets/letsencrypt-certs
	@openssl ecparam -name secp521r1 -genkey -out ./config/minikube/secrets/letsencrypt-certs/tls.key      
	@openssl req -new -x509 -key ./config/minikube/secrets/letsencrypt-certs/tls.key -out ./config/minikube/secrets/letsencrypt-certs/tls.crt -days 3652 -config ./config/minikube/openssl-letsencrypt.cnf

config/minikube/secrets/nginx-ingress/tls.key config/minikube/secrets/nginx-ingress/tls.crt: config/minikube/openssl-nginx.cnf
	@mkdir -p config/minikube/secrets/nginx-ingress
	@openssl ecparam -name secp521r1 -genkey -out ./config/minikube/secrets/nginx-ingress/tls.key
	@openssl req -new -x509 -key ./config/minikube/secrets/nginx-ingress/tls.key -out ./config/minikube/secrets/nginx-ingress/tls.crt -days 3652 -config ./config/minikube/openssl-nginx.cnf

config/minikube/secrets/mocks/tls.key config/minikube/secrets/mocks/tls.crt: config/minikube/openssl-mocks.cnf
	@mkdir -p config/minikube/secrets/mocks
	@openssl ecparam -name secp521r1 -genkey -out ./config/minikube/secrets/mocks/tls.key
	@openssl req -new -x509 -key ./config/minikube/secrets/mocks/tls.key -out ./config/minikube/secrets/mocks/tls.crt -days 3652 -config ./config/minikube/openssl-mocks.cnf

config/minikube/secrets/slack/slack-client-secret config/minikube/secrets/slack/slack-signing-secret: out/bin/mocks/kms
	@mkdir -p config/minikube/secrets/slack
	./out/bin/mocks/kms -cli -encrypt=slack-client-secret-value > ./config/minikube/secrets/slack/slack-client-secret
	./out/bin/mocks/kms -cli -encrypt=slack-signing-secret-value > ./config/minikube/secrets/slack/slack-signing-secret

config/minikube/secrets/google/k8s-dice-magic.json:
	@mkdir -p config/minikube/secrets/google
	curl https://gist.githubusercontent.com/aasmall/b9cb7a5d6c3c675d7c99b3c19082d4c8/raw/83bcaa1667546d74f68cb04c6442d8a4306a0a12/fake-google-creds.json -o ./config/minikube/secrets/google/k8s-dice-magic.json

.PHONY: secrets
secrets: config/minikube/secrets/slack/slack-client-secret config/minikube/secrets/slack/slack-signing-secret config/minikube/secrets/mocks/tls.key config/minikube/secrets/mocks/tls.crt config/minikube/secrets/nginx-ingress/tls.key config/minikube/secrets/nginx-ingress/tls.crt config/minikube/secrets/letsencrypt-certs/tls.key config/minikube/secrets/letsencrypt-certs/tls.crt config/minikube/secrets/google/k8s-dice-magic.json

# .PHONY: docker $(NAMES) $(IMAGES)
# 	@echo DOCKERFILES: $(DOCKERFILES)
# 	@echo NAMES: $(NAMES)
# 	@echo REGISTRY: $(REGISTRY)
# 	@echo IMAGES: $(IMAGES)
# 	@echo -------------

# # $(NAMES): %: $(REGISTRY)/%
# 	docker push $<

# $(IMAGES): %:
# 	docker build --build-arg REGISTRY=$(REGISTRY) -t $@ $(subst :,/,$(subst $(REGISTRY)/,,$@))
	

# .PHONY: $(IMAGENAMES)
# $(IMAGENAMES): $(DOCKERFILES)
# 	docker build . -f $< -t registry.gitlab.com/aasmall/dicemagic/$(<F)
# 	docker push registry.gitlab.com/aasmall/dicemagic/$(<@):latest

# .PHONY: builder
# builder: 
# 	docker build . -f ./dockerfiles/builder.dockerfile -t registry.gitlab.com/aasmall/dicemagic/builder
# 	docker push registry.gitlab.com/aasmall/dicemagic/builder:latest

# .PHONY: chat-clients-image
# chat-clients: builder-image
# 	docker build . -f ./dockerfiles/chat-clients.dockerfile -t registry.gitlab.com/aasmall/dicemagic/chat-clients
# 	docker push registry.gitlab.com/aasmall/dicemagic/chat-clients:latest

# .PHONY: dice-server-image
# chat-clients: builder
# 	docker build . -f ./dockerfiles/dice-server.dockerfile -t registry.gitlab.com/aasmall/dicemagic/dice-server
# 	docker push registry.gitlab.com/aasmall/dicemagic/dice-server:latest

# .PHONY: letsencrypt-image
# chat-clients: builder-image
# 	docker build . -f ./dockerfiles/letsencrypt.dockerfile -t registry.gitlab.com/aasmall/dicemagic/letsencrypt
# 	docker push registry.gitlab.com/aasmall/dicemagic/letsencrypt:latest

# .PHONY: mock-datastore-image
# chat-clients: builder-image
# 	docker build . -f ./dockerfiles/mock-datastore.dockerfile -t registry.gitlab.com/aasmall/dicemagic/mock-datastore
# 	docker push registry.gitlab.com/aasmall/dicemagic/mock-datastore:latest
	
# .PHONY: mock-kms-image
# chat-clients: builder-image
# 	docker build . -f ./dockerfiles/mock-kms.dockerfile -t registry.gitlab.com/aasmall/dicemagic/mock-kms
# 	docker push registry.gitlab.com/aasmall/dicemagic/mock-kms:latest

# .PHONY: mock-slack-server
# chat-clients: builder
# 	docker build . -f ./dockerfiles/mock-slack-server.dockerfile -t registry.gitlab.com/aasmall/dicemagic/mock-slack-server
# 	docker push registry.gitlab.com/aasmall/dicemagic/mock-slack-server:latest

# .PHONY: redis
# chat-clients: builder
# 	docker build . -f ./dockerfiles/redis.dockerfile -t registry.gitlab.com/aasmall/dicemagic/redis
# 	docker push registry.gitlab.com/aasmall/dicemagic/redis:latest

# .PHONY: www
# chat-clients: builder
# 	docker build . -f ./dockerfiles/www.dockerfile -t registry.gitlab.com/aasmall/dicemagic/www
# 	docker push registry.gitlab.com/aasmall/dicemagic/www:latest

clean:
	rm -rf ./out/ ./config/minikube/secrets/
	rm lib/dicelang/dicelang.pb.go chat-clients/channeltype_string.go mocks/slack-server/clienttype_string.go
	