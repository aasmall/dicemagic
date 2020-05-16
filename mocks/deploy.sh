#!/bin/bash
pushd "${0%/*}" > /dev/null
kubectl delete all -l app=mocks
kubectl apply -f mocks.yaml
popd > /dev/null