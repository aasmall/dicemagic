#!/bin/bash

if [[ -z "${SLACK_CLIENT_ID}" ]]; then
  echo "Provide Slack CLIENT ID for development Slack App"
  read SLACK_CLIENT_ID
  export SLACK_CLIENT_ID=${SLACK_CLIENT_ID}
fi

if [[ -z "${SLACK_APP_ID}" ]]; then
echo "Provide Slack APP ID for development Slack App"
read SLACK_APP_ID
export SLACK_APP_ID=${SLACK_APP_ID}
fi

rm -rf ./www
mkdir www && cp ../www/config.toml ./www/config.toml
sed -i -e "s/42142079431.351057397637/${SLACK_CLIENT_ID?}/g" ./www/config.toml

declare -a builds=("cloudbuild.dev.chat-clients.yaml" "cloudbuild.dev.dice-server.yaml" "cloudbuild.dev.letsencrypt.yaml" "cloudbuild.dev.redis.yaml" "cloudbuild.dev.www.yaml")
declare -a pids=()
# run processes and store pids in array
for i in ${builds[@]}; do
    echo "running builds for ${i}"
    gcloud builds submit ../ --config=build-files/${i} &
    pids+=($!)
done


# wait for all pids
for pid in ${pids[*]}; do
    echo "wait for pid: ${pid}"
    wait $pid
done

echo "done"

rm -rf ./k8s
mkdir k8s && cp ../k8s/*.yaml ./k8s
for filename in ./k8s/*.yaml; do
    sed -i -e 's/api.dicemagic.io/api.dev.dicemagic.io/g' ${filename}
    sed -i -e 's/www.dicemagic.io/www.dev.dicemagic.io/g' ${filename}
    sed -i -e 's/gcr.io\/k8s-dice-magic\/chat-clients:latest/gcr.io\/k8s-dice-magic\/chat-clients-dev:latest/g' ${filename}
    sed -i -e 's/gcr.io\/k8s-dice-magic\/dice-server:latest/gcr.io\/k8s-dice-magic\/dice-server-dev:latest/g' ${filename}
    sed -i -e 's/gcr.io\/k8s-dice-magic\/letsencrypt:latest/gcr.io\/k8s-dice-magic\/letsencrypt-dev:latest/g' ${filename}
    sed -i -e 's/gcr.io\/k8s-dice-magic\/www:latest/gcr.io\/k8s-dice-magic\/www-dev:latest/g' ${filename}
    sed -i -e 's/kubernetes.io\/ingress.global-static-ip-name: "dice-magic"/kubernetes.io\/ingress.global-static-ip-name: "dice-magic-dev"/g' ${filename}
    sed -i -e "s/42142079431.351057397637/${SLACK_CLIENT_ID?}/g" ${filename}
    sed -i -e "s/AAB1PBPJR/${SLACK_APP_ID?}/g" ${filename}
    sed -i -e "s/35.226.187.207/35.202.172.148/g" ${filename}
    sed -i -e "s/DEBUG: \"false\"/DEBUG: \"true\"/g" ${filename}
    sed -i -e "s/LOG_NAME: \"dicemagic-logs\"/LOG_NAME: \"dicemagic-dev-logs\"/g" ${filename}
done
kubectl delete pods --all
kubectl apply -f ./k8s
rm -rf ./k8s
rm -rf ./www