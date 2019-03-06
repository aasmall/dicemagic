#!/bin/bash
echo "Provide Slack CLIENT SECRET for development Slack App"
read CLEAR_SLACK_CLIENT_SECRET
echo "Provide Slack SIGNING SECRET for development Slack App"
read CLEAR_SLACK_SIGNING_SECRET
echo "Provide Slack CLIENT ID for development Slack App"
read SLACK_CLIENT_ID
export SLACK_CLIENT_ID=${SLACK_CLIENT_ID}
echo "Provide Slack APP ID for development Slack App"
read SLACK_APP_ID
export SLACK_APP_ID=${SLACK_APP_ID}

rm -rf ./secrets
mkdir secrets
printf ${CLEAR_SLACK_CLIENT_SECRET} | tee ./secrets/slack-client-secret.txt >/dev/null 2>&1 && unset CLEAR_SLACK_CLIENT_SECRET
printf ${CLEAR_SLACK_SIGNING_SECRET} | tee ./secrets/slack-signing-secret.txt >/dev/null 2>&1 && unset CLEAR_SLACK_SIGNING_SECRET

SLACK_CLIENT_SECRET=$(gcloud kms encrypt --location=us-central1 --keyring=dice-magic --key=slack --plaintext-file=./secrets/slack-client-secret.txt --ciphertext-file=- | base64 -w0)
SLACK_SIGNING_SECRET=$(gcloud kms encrypt --location=us-central1 --keyring=dice-magic --key=slack --plaintext-file=./secrets/slack-signing-secret.txt --ciphertext-file=- | base64 -w0)
kubectl create secret generic slack-secrets --from-literal=slack-client-secret=${SLACK_CLIENT_SECRET} --from-literal=slack-signing-secret=${SLACK_SIGNING_SECRET}
rm ./secrets/slack-client-secret.txt && rm ./secrets/slack-signing-secret.txt

mkdir ./cleanup

gcloud iam service-accounts keys create ./secrets/k8s-dice-magic.json --iam-account=dice-magic-app@k8s-dice-magic.iam.gserviceaccount.com >> ./cleanup/keyids.txt 2>&1
gcloud iam service-accounts keys create ./secrets/certbot-dns.json --iam-account=certbot-dns-manager@k8s-dice-magic.iam.gserviceaccount.com >> ./cleanup/keyids.txt 2>&1

kubectl create secret generic google-default --from-file=./secrets/k8s-dice-magic.json 
kubectl create secret generic certbot-dns --from-file=./secrets/certbot-dns.json 
rm ./secrets/k8s-dice-magic.json
rm ./secrets/certbot-dns.json
rm -r ./secrets

. ./update-pods.sh

kubectl apply -f certbot-first-run.yaml