FROM google/cloud-sdk:alpine
RUN apk --update add openjdk11-jre socat
RUN gcloud components install beta cloud-datastore-emulator --quiet
ENV CLOUDSDK_CORE_PROJECT="dice-magic-minikube" 
ENV DATASTORE_EMULATOR_HOST=localhost:40081
COPY ./out/bin/mocks/datastore /usr/bin/update-datastore
COPY ./out/include/mocks/datastore/bootstrap-datastore.sh /usr/bin/bootstrap-datastore.sh
RUN chmod +x /usr/bin/bootstrap-datastore.sh
ENTRYPOINT /usr/bin/bootstrap-datastore.sh