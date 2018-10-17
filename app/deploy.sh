#!/bin/bash
gcloud builds submit . \
    --config=cloudbuild.yaml \
    && kubectl delete pods --selector=app=dice-magic-app \
    && kubectl get pods --watch