#!/bin/bash
gcloud builds submit . \
    --config=cloudbuild.yaml \
    && kubectl delete pods --selector=app=www \
    && kubectl get pods --watch