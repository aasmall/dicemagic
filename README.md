# MagicDice
Slack Bot for Rolling Complex Dice

#set up dev environment
export SLACK_KEY=$"CiQAgjBGQm1aFTfNsTYceo46b1omBy+BvZjPPOIitEyK6opxWckSSQCdk3ogbDHvTEHHMD8V+QmKRq65dhQPaU5Fh+OsUSunVgI2HKZMFIviuxqKQs3YgrwuVALIQ3476V4VB42gaNViw7mG2/QFkns="
export SLACK_CLIENT_SECRET=$"CiQAgjBGQt3nuR5YR96644L9AzUKR3TKpoVhTwmULGeAGCi850ISQACdk3og8Bc5uTs9YzYiDRUGqdbphyn3MjgEYQytT6HANZtJEs7bthIzjOtAIlMH4XyFfhMERH6Jasbrl55Hyrg="
export PROJECT_ID=$"dice-magic"
export KMSKEYRING=$"DiceMagic"
export KMSKEY=$"SlackBotKeyKEK"
export GOOGLE_APPLICATION_CREDENTIALS="/mnt/c/Users/aaron/aasmall/MagicDice/sa.json"
export GOPATH=$"/mnt/c/Users/aaron/aasmall/go/"
go get
dev_appserver.py app.yaml