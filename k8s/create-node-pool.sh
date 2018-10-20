#!/bin/bash
NODE_SA_EMAIL=gke-node-sa@k8s-dice-magic.iam.gserviceaccount.com
gcloud beta container node-pools create dice-pool \
  --cluster=dice-magic \
  --workload-metadata-from-node=SECURE \
  --service-account=$NODE_SA_EMAIL \
  --metadata disable-legacy-endpoints=true \
  --enable-autorepair \
  --num-nodes=3 \
  --enable-autoupgrade \
  -m g1-small
gcloud beta container node-pools create micro-dice-pool \
  --cluster=dice-magic \
  --workload-metadata-from-node=SECURE \
  --service-account=$NODE_SA_EMAIL \
  --metadata disable-legacy-endpoints=true \
  --enable-autorepair \
  --num-nodes=1 \
  --enable-autoupgrade \
  -m f1-micro \
