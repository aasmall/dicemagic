package main

import (
	"reflect"
	"testing"
)

func Test_stringToColor(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name string
		args args
		want string
	}{{name: "random color",
		args: args{input: "random"},
		want: "#2F164D"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringToColor(tt.args.input); got != tt.want {
				t.Errorf("stringToColor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollDecision_ToSlackAttachment(t *testing.T) {
	tests := []struct {
		name     string
		decision *RollDecision
		want     Attachment
		wantErr  bool
	}{{name: "Ravenloft",
		decision: &RollDecision{
			question: "Should we go to Ravenloft or stay here?",
			choices:  []string{"go to raventloft", "stay here"},
			result:   0},
		want:    Attachment{},
		wantErr: false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.decision.ToSlackAttachment()
			if (err != nil) != tt.wantErr {
				t.Errorf("RollDecision.ToSlackAttachment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RollDecision.ToSlackAttachment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollExpression_ToSlackAttachment(t *testing.T) {
	tests := []struct {
		name       string
		expression *RollExpression
		want       Attachment
		wantErr    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.expression.ToSlackAttachment()
			if (err != nil) != tt.wantErr {
				t.Errorf("RollExpression.ToSlackAttachment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RollExpression.ToSlackAttachment() = %v, want %v", got, tt.want)
			}
		})
	}
}
