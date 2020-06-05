package errors

import (
	"errors"
	"fmt"
)

const (
	// InvalidAST is the error type that occurs when the AST that resulted from the command is invalid in some way. Honestly, this shouldn't happen.
	InvalidAST = iota
	// InvalidCommand is the error type that occurs when a command cannot be parsed into an AST
	InvalidCommand
	// Friendly represents an expected error
	Friendly
	// Unexpected errors should not occur.
	Unexpected = 999
)

//LexError represents an error occured during parsing of a dicelang statement.
type LexError struct {
	Err  string
	Col  int
	Line int
}

//Error returns the message string
func (e LexError) Error() string {
	return e.Err
}

//NewLexError creates a new LexError
func NewLexError(text string, col int, line int) *LexError {
	return &LexError{
		Err:  text,
		Col:  col,
		Line: line,
	}
}

//DicelangError represents a custom error thrown by Dicelang
type DicelangError struct {
	Err   string
	Code  int32
	Inner error
}

//Error returns the message string
func (e DicelangError) Error() string {
	return e.Err
}

//NewDicelangError creates a new DiceLangError
func NewDicelangError(text string, code int32, inner error) *DicelangError {
	return &DicelangError{
		Err:   text,
		Code:  code,
		Inner: inner,
	}
}

//New creates a new simple error
func New(text string) error {
	return errors.New(text)
}

//Newf creates a new simple error with fmt.Sprintf
func Newf(text string, a ...interface{}) error {
	return fmt.Errorf(text, a...)
}
