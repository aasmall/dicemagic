#!/bin/bash
gcloud builds submit . \
    --config=cloudbuild.local.yaml \
    && kubectl delete pods --all \
    && kubectl get pods --watch