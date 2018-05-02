# MagicDice

Slack Bot for Rolling Complex Dice

## Set up dev environment

requires:
 - [Google Cloud SDK](https://cloud.google.com/sdk/downloads)
 - [Go](https://golang.org/doc/install)
 - [AppEngine default service account key.json](https://console.cloud.google.com/iam-admin/serviceaccounts/project)
 - Python 
    $ sudo apt-get install python

```bash
#You'll need a private key to access the AppEngine Service Account. You probably don't have this
export GOOGLE_APPLICATION_CREDENTIALS="/mnt/c/Users/aaron/aasmall/MagicDice/sa.json"

go get
dev_appserver.py app.yaml
```

## How to encrypt new secrets

```bash
echo -n "Some text to be encrypted" | base64

curl -s -X POST "https://cloudkms.googleapis.com/v1/projects/dice-magic/locations/global/keyRings/DiceMagic/cryptoKeys/SlackBotKeyKEK/cryptoKeyVersions/1:encrypt" \
  -d "{\"plaintext\":\"[base_64_encoded_secret_goes_here]"}" \
  -H "Authorization:Bearer $(gcloud auth print-access-token)" \
  -H "Content-Type:application/json"
```