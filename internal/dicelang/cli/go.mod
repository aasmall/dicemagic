module github.com/aasmall/dicemagic/internal/dicelang/cli

go 1.12

require (
	github.com/aasmall/dicemagic/internal/dicelang v0.1.0
	github.com/aasmall/dicemagic/internal/dicelang/errors v0.1.0
)

replace github.com/aasmall/dicemagic/internal/dicelang v0.1.0 => ../

replace github.com/aasmall/dicemagic/internal/dicelang/errors v0.1.0 => ../errors
