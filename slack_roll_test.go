package main

import "testing"

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
