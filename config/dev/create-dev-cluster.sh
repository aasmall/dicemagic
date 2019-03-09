#!/bin/bash
echo "Make sure you have created a new project and linked a billing account"
echo "Make sure you have created a datastore database in datastore mode"
echo "Provide the Project ID you wish to create a new cluster in"
read PROJECT_ID
gcloud config configurations create ${PROJECT_ID?} || exit 1
gcloud config set project ${PROJECT_ID?} || exit 1

echo "Provide new Cluster Name"
read CLUSTER_NAME

echo "Provide the dns name for the new service. Clearly, you must own this."
echo "For automation to work, Google must be your NS"
read DNS_NAME

echo "Provide a GCP zone to create the cluster in. e.g. \"us-central1-a\""
read ZONE
gcloud config set compute/zone ${ZONE?} || exit 1

gcloud auth login

gcloud iam service-accounts create node-service-account --display-name node-service-account
gcloud iam service-accounts create dns-service-account --display-name dns-service-account
gcloud iam service-accounts create dice-magic-service-account --display-name dice-magic-service-account
# Grant roles to node service account
declare -a roles=("roles/logging.logWriter" "roles/monitoring.metricWriter" "roles/monitoring.viewer" "roles/storage.objectViewer")
for i in ${roles[@]}; do
gcloud projects add-iam-policy-binding ${PROJECT_ID?} \
--member=serviceAccount:node-service-account@${PROJECT_ID?}.iam.gserviceaccount.com \
--role=${i}
done
# Grant roles to app service account
roles=("roles/cloudkms.cryptoKeyDecrypter" "roles/cloudkms.cryptoKeyEncrypter" "roles/cloudprofiler.agent" "roles/cloudtrace.agent" "roles/datastore.user" "roles/logging.logWriter" "roles/monitoring.metricWriter")
for i in ${roles[@]}; do
gcloud projects add-iam-policy-binding ${PROJECT_ID?} \
--member=serviceAccount:dice-magic-service-account@${PROJECT_ID?}.iam.gserviceaccount.com \
--role=${i}
done
# Grant role to dns service account
gcloud projects add-iam-policy-binding ${PROJECT_ID?} \
--member=serviceAccount:dns-service-account@${PROJECT_ID?}.iam.gserviceaccount.com \
--role=roles/dns.admin

# Enable required services
declare -a services=("container.googleapis.com" "cloudkms.googleapis.com" "cloudbuild.googleapis.com" "dns.googleapis.com" "storage-component.googleapis.com")
declare -a servicepids=()
for i in ${services[@]}; do
    echo "enabling ${i}"
    gcloud services enable ${i} &
    servicepids+=($!)
done
for pid in ${servicepids[*]}; do
    echo "wait for pid: ${pid}"
    wait $pid
done


gcloud beta container --project "${PROJECT_ID?}" clusters create "${CLUSTER_NAME?}" \
--zone "${ZONE?}" \
--no-enable-basic-auth \
--cluster-version "1.12.5-gke.5" \
--machine-type "g1-small" \
--image-type "COS" \
--disk-type "pd-standard" \
--disk-size "100" \
--metadata disable-legacy-endpoints=true \
--service-account "node-service-account@${PROJECT_ID?}.iam.gserviceaccount.com" \
--preemptible --num-nodes "3" --enable-cloud-logging --enable-cloud-monitoring --no-enable-ip-alias \
--addons HorizontalPodAutoscaling,HttpLoadBalancing --enable-autoupgrade --enable-autorepair

gcloud container clusters get-credentials ${CLUSTER_NAME?} 

#create static IP
gcloud compute addresses create dice-magic --global --ip-version=IPV4
export IP=$(gcloud compute addresses describe dice-magic --global | grep -Eo "\b([0-9]{1,3}\.){3}[0-9]{1,3}\b")

#create zone and dns entries
gcloud beta dns --project=${PROJECT_ID?} managed-zones create dicemagic-dev --description= --dns-name=${DNS_NAME}.
gcloud dns --project=${PROJECT_ID?} record-sets transaction start --zone=dicemagic-dev
gcloud dns --project=${PROJECT_ID?} record-sets transaction add ${IP} --name=${DNS_NAME}. --ttl=300 --type=A --zone=dicemagic-dev
gcloud dns --project=${PROJECT_ID?} record-sets transaction add ${DNS_NAME}. --name=api.${DNS_NAME}. --ttl=300 --type=CNAME --zone=dicemagic-dev
gcloud dns --project=${PROJECT_ID?} record-sets transaction add ${DNS_NAME}. --name=www.${DNS_NAME}. --ttl=300 --type=CNAME --zone=dicemagic-dev
gcloud dns --project=${PROJECT_ID?} record-sets transaction execute --zone=dicemagic-dev
echo "----------------------------------------"
echo "use IP: ${IP} to update ./k8s/overlays/dev-cluster/nginx-ingress-controller.yaml"
echo "----------------------------------------"