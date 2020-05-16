#!/bin/bash
pushd "${0%/*}" > /dev/null
files=( "chat-clients" "dice-server" "www")
mkdir bin
for i in "${files[@]}"
do
    pushd $i > /dev/null
    echo "Building $i at $PWD"
    go build
    echo "Docker Build $i at $PWD"
    docker build -f slim.dockerfile -t $(minikube ip):5000/$i:latest .
    docker push $(minikube ip):5000/$i:latest
    popd > /dev/null
done
popd > /dev/null