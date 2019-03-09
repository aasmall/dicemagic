#!/bin/bash
echo "Provide Slack CLIENT SECRET for development Slack App"
read CLEAR_SLACK_CLIENT_SECRET
echo "Provide Slack SIGNING SECRET for development Slack App"
read CLEAR_SLACK_SIGNING_SECRET
echo "Provide GCP region for keyring e.g. \"us-central1\""
read REGION
echo "Provide the Project ID you wish to create a new cluster in"
read PROJECT_ID

gcloud kms keyrings create dice-magic --location ${REGION?}
gcloud kms keys create slack --purpose=encryption --keyring dice-magic --location=${REGION?}

rm -rf ./secrets
mkdir secrets
printf ${CLEAR_SLACK_CLIENT_SECRET} | tee ./secrets/slack-client-secret.txt >/dev/null 2>&1 && unset CLEAR_SLACK_CLIENT_SECRET
printf ${CLEAR_SLACK_SIGNING_SECRET} | tee ./secrets/slack-signing-secret.txt >/dev/null 2>&1 && unset CLEAR_SLACK_SIGNING_SECRET

SLACK_CLIENT_SECRET=$(gcloud kms encrypt --location=${REGION?} --keyring=dice-magic --key=slack --plaintext-file=./secrets/slack-client-secret.txt --ciphertext-file=- | base64 -w0)
SLACK_SIGNING_SECRET=$(gcloud kms encrypt --location=${REGION?} --keyring=dice-magic --key=slack --plaintext-file=./secrets/slack-signing-secret.txt --ciphertext-file=- | base64 -w0)
kubectl delete secret slack-secrets
kubectl create secret generic slack-secrets --from-literal=slack-client-secret=${SLACK_CLIENT_SECRET} --from-literal=slack-signing-secret=${SLACK_SIGNING_SECRET}
rm ./secrets/slack-client-secret.txt && rm ./secrets/slack-signing-secret.txt

gcloud iam service-accounts keys create ./secrets/k8s-dice-magic.json --iam-account=dice-magic-service-account@${PROJECT_ID?}.iam.gserviceaccount.com >> ./cleanup/keyids.txt 2>&1
gcloud iam service-accounts keys create ./secrets/certbot-dns.json --iam-account=dns-service-account@${PROJECT_ID?}.iam.gserviceaccount.com >> ./cleanup/keyids.txt 2>&1

kubectl delete secret google-default
kubectl create secret generic google-default --from-file=./secrets/k8s-dice-magic.json 
kubectl delete secret certbot-dns
kubectl create secret generic certbot-dns --from-file=./secrets/certbot-dns.json 
rm ./secrets/k8s-dice-magic.json
rm ./secrets/certbot-dns.json
rm -r ./secrets

echo "update dev-cluster overlays with correct projectID in image tags"
read  -n 1 -p "Then press any key"
. ./update-pods.sh

kubectl apply -f certbot-first-run.yaml