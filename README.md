# MagicDice
Slack Bot for Rolling Complex Dice

## Set up dev environment:
```
#You'll need a private key to access the AppEngine Service Account. You probably don't have this
export GOOGLE_APPLICATION_CREDENTIALS="/mnt/c/Users/aaron/aasmall/MagicDice/sa.json"

#Set your GOPATH to your workspace
export GOPATH=$"/mnt/c/Users/aaron/aasmall/go/"

go get
dev_appserver.py app.yaml
```