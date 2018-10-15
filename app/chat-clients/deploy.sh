#!/bin/bash
gcloud builds submit . \
    --config=cloudbuild.yaml \
    && kubectl delete pods --all \
    && kubectl get pods --watch