#!/bin/bash
rm -rf ./www
mkdir www
/snap/bin/yq m -x "../../www/config.yaml" "www-config.yaml" >./www/config.yaml

declare -a builds=("cloudbuild.dev.chat-clients.yaml" "cloudbuild.dev.dice-server.yaml" "cloudbuild.dev.letsencrypt.yaml" "cloudbuild.dev.redis.yaml" "cloudbuild.dev.www.yaml")
declare -a pids=()
# run processes and store pids in array
for i in ${builds[@]}; do
    echo "running builds for ${i}"
    gcloud builds submit ../../ --config=build-files/${i} &
    pids+=($!)
done

# wait for all pids
for pid in ${pids[*]}; do
    echo "wait for pid: ${pid}"
    wait $pid
done

echo "done"

kubectl delete pods --all
kustomize build ../k8s/overlays/dev-cluster | kubectl apply -f -
rm -rf ./www