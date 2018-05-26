#!/bin/bash
gcloud app deploy cron.yaml app.yaml queue.yaml ./api/api.yaml ./www/www.yaml ./worker/worker.yaml