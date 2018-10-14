#!/bin/bash
kubectl create secret generic google-default --from-file=./k8s-dice-magic.json
kubectl create secret generic certbot-dns --from-file=./certbot-dns.json
kubectl create secret generic slack-secrets --from-literal=slack-client-secret=$(gcloud kms encrypt \
    --location=us-central1 \
    --keyring=dice-magic \
    --key=slack \
    --plaintext-file=slack-client-secret.txt \
    --ciphertext-file=- \
    | base64 -w0) \
    --from-literal=slack-signing-secret=$(gcloud kms encrypt \
    --location=us-central1 \
    --keyring=dice-magic \
    --key=slack \
    --plaintext-file=slack-signing-secret.txt \
    --ciphertext-file=- \
    | base64 -w0)