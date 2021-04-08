FROM golang:alpine3.13
RUN apk add --no-cache \
            build-base make git
COPY . /dicemagic
WORKDIR /dicemagic
RUN go get -u golang.org/x/lint/golint
RUN go get github.com/gohugoio/hugo && go install github.com/gohugoio/hugo
RUN go get ./...

FROM golang:buster
RUN apt-get update && apt-get install -y --no-install-recommends \
    unzip \
    && rm -rf /var/lib/apt/lists/*
COPY --from=0 /go/pkg/ /go/pkg/