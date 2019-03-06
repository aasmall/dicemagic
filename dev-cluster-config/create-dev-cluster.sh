#!/bin/bash
PROJECT_ID=$(gcloud config get-value project)

gcloud beta container --project "${PROJECT_ID?}" clusters create "dice-magic-dev" \
--zone "us-central1-a" \
--no-enable-basic-auth \
--cluster-version "1.12.5-gke.5" \
--machine-type "g1-small" \
--image-type "COS" \
--disk-type "pd-standard" \
--disk-size "100" \
--metadata disable-legacy-endpoints=true \
--service-account "gke-node-sa@k8s-dice-magic.iam.gserviceaccount.com" \
--preemptible --num-nodes "3" --enable-cloud-logging --enable-cloud-monitoring --no-enable-ip-alias \
--network "projects/k8s-dice-magic/global/networks/default" \
--subnetwork "projects/k8s-dice-magic/regions/us-central1/subnetworks/default" \
--addons HorizontalPodAutoscaling,HttpLoadBalancing --enable-autoupgrade --enable-autorepair

gcloud container clusters get-credentials dice-magic-dev
