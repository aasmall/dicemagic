package errors

import (
	"reflect"
	"testing"
)

func TestLexError_Error(t *testing.T) {
	type fields struct {
		Err  string
		Col  int
		Line int
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "oh no!",
			fields: fields{Err: "oh no!", Col: 1, Line: 1},
			want:   "oh no!",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := LexError{
				Err:  tt.fields.Err,
				Col:  tt.fields.Col,
				Line: tt.fields.Line,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("LexError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewLexError(t *testing.T) {
	type args struct {
		text string
		col  int
		line int
	}
	tests := []struct {
		name string
		args args
		want *LexError
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewLexError(tt.args.text, tt.args.col, tt.args.line); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewLexError() = %v, want %v", got, tt.want)
			}
		})
	}
}
