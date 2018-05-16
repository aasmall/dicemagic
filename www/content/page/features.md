# Features

### 1) Cryptographically strong random number generation

Dice Magic uses Go's [crypto/rand](https://golang.org/pkg/crypto/rand/) package to roll dice. It's overkill, but nice to know.

### 2) Implements most standard notation and more

Will interpret pretty much any standard notation dice roll, such as 1d20 or 1d8+5, but also allows for the addition of roll types like 
`roll 1d20 attack and 1d8+3 mundane`

### 3) Remembers you

Dice Magic lets you remember and replay common rolls: `remember atk1 roll 1d20+5 attack then 1d12+5 mundane` followed by `!atk1`. You can also use the `!!` command to replay the last roll you made.

### 4) Responds to mentions

You can DM Dice Magic for most things, or @mention it in a channel to broadcast your rolls.

### 5) Fast and getting faster

Dice Magic responds in under 200ms in most cases. This is a pet project for me to learn Go and AppEngine, so it will probably get faster over time.
