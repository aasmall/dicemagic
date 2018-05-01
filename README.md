# MagicDice

Slack Bot for Rolling Complex Dice

## Set up dev environment

```bash
#You'll need a private key to access the AppEngine Service Account. You probably don't have this
export GOOGLE_APPLICATION_CREDENTIALS="/mnt/c/Users/aaron/aasmall/MagicDice/sa.json"

#Set your GOPATH to your workspace
export GOPATH=$"/mnt/c/Users/aaron/aasmall/go/"

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