#!/bin/bash
kubectl create secret generic google-default --from-file=./k8s-dice-magic.json
kubectl create secret generic certbot-dns --from-file=./certbot-dns.json