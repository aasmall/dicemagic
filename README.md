
# Dice Magic

Slack Bot for Rolling Complex Dice


|Branch|Build Status |
|--|--|
| master | [![Build Status](https://semaphoreci.com/api/v1/aasmall/dicemagic/branches/master/badge.svg)](https://semaphoreci.com/aasmall/dicemagic) |
| vDev |  [![Build Status](https://semaphoreci.com/api/v1/aasmall/dicemagic/branches/vdev/badge.svg)](https://semaphoreci.com/aasmall/dicemagic)|

## Set up dev environment

requires:
 - [Google Cloud SDK](https://cloud.google.com/sdk/downloads)
 - [Go](https://golang.org/doc/install)
 - [AppEngine default service account key.json](https://console.cloud.google.com/iam-admin/serviceaccounts/project)
 - Python 2.7  `sudo apt-get install python`
 - MagicDice in `$GOPATH/src/github.com/aasmall/`
 - GOOGLE_APPLICATION_CREDENTIALS environment variable set
 - [AppEngine API Enabled] (https://console.cloud.google.com/apis/library/appengine.googleapis.com/)

```bash
export GOOGLE_APPLICATION_CREDENTIALS="[PATH TO SERVICE ACCOUNT JSON]"
```

### Example Folder Structure

```bash
$GOPATH
├── bin
├── pkg
└── src
    └── github.com
        └── aasmall
            └── MagicDice
```

### Example Build Script

```bash
wget -O ~/sdk-tar.gz \
https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-200.0.0-linux-x86_64.tar.gz
tar -xzf ~/sdk-tar.gz -C ~/
~/google-cloud-sdk/install.sh --quiet
. ~/google-cloud-sdk/path.bash.inc
gcloud components update --quiet
gcloud components install app-engine-go --quiet
cd $GOPATH/src/github.com/aasmall/MagicDice
go get -d -v -t ./... && go build -v ./...

```

