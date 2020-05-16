module github.com/aasmall/dicemagic/lib/dicelang/cli

go 1.14

require (
	github.com/aasmall/dicemagic/lib/dicelang v0.1.0
	github.com/aasmall/dicemagic/lib/dicelang/errors v0.1.0
)

replace github.com/aasmall/dicemagic/lib/dicelang v0.1.0 => ../

replace github.com/aasmall/dicemagic/lib/dicelang/errors v0.1.0 => ../errors
