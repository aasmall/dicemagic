#!/bin/bash
set -m

gcloud beta emulators datastore start --no-store-on-disk --verbosity=debug --host-port=localhost:40081 &
while true; do
    RESPONSE=$(curl --write-out %{http_code} --silent --output /dev/null  http://localhost:40081)
    if (("$RESPONSE" == "200")); then
        break
    fi
done
$(gcloud beta emulators datastore env-init)
sleep 1
socat tcp-l:50081,fork,reuseaddr tcp:127.0.0.1:40081 &
/usr/bin/update-datastore &
echo "Update-Datastore Running"
jobs
fg %1