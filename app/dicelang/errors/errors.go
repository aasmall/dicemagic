package errors

import "errors"

const (
	InvalidAST = iota
	InvalidCommand
	Friendly
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
