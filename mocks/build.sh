#!/bin/bash
pushd "${0%/*}" > /dev/null
files=( "kms" "slack-server")
mkdir bin
for i in "${files[@]}"
do
    pushd $i > /dev/null
    echo "Building $i at $PWD"
    go build
    echo "Docker Build $i at $PWD"
    docker build -f slim.dockerfile -t localhost:5000/$i:latest .
    docker push localhost:5000/$i:latest
    popd > /dev/null
done
cd datastore
docker build -f slim.dockerfile -t localhost:5000/datastore:latest .
docker push localhost:5000/datastore:latest
popd > /dev/null