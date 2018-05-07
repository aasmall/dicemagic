package internal

import (
	"reflect"
	"strings"
	"testing"
)

func TestRollExpression_GetTotalsByType(t *testing.T) {
	tests := []struct {
		name    string
		r       *RollExpression
		want    []RollTotal
		wantErr bool
	}{
		{name: "fobar",
			r: NewParser(strings.NewReader("ROLL (21d1+7)/2[mundane]+4d1[fire]")).MustParse(),
			want: []RollTotal{
				{RollType: "Fire", RollResult: 4},
				{RollType: "Mundane", RollResult: 14}},
			wantErr: false},
		{name: "fobar",
			r: NewParser(strings.NewReader("ROLL (8d1+10)*2+5[mundane]+6d1/2[fire]")).MustParse(),
			want: []RollTotal{
				{RollType: "Fire", RollResult: 3},
				{RollType: "Mundane", RollResult: 41}},
			wantErr: false},
		{name: "No Types",
			r: NewParser(strings.NewReader("ROLL (8d1+10)*2+5")).MustParse(),
			want: []RollTotal{
				{RollType: "", RollResult: 41}},
			wantErr: false},
		{name: "mixmatched types",
			r: NewParser(strings.NewReader("ROLL (8d1+10)*2+5[mundane]+6d1/2*5-10")).MustParse(),
			want: []RollTotal{
				{RollType: "", RollResult: 5},
				{RollType: "Mundane", RollResult: 41}},
			wantErr: false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.r.GetTotalsByType()
			if (err != nil) != tt.wantErr {
				t.Errorf("RollExpression.GetTotalsByType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RollExpression.GetTotalsByType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollExpression_String(t *testing.T) {
	tests := []struct {
		name string
		r    *RollExpression
		want string
	}{
		{name: "ReString",
			r:    NewParser(strings.NewReader("ROLL (21D1+7)/2[mundane]+4d1[fire]")).MustParse(),
			want: "Roll (21d1+7)/2[Mundane]+4d1[Fire]"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.String(); got != tt.want {
				t.Errorf("RollExpression.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
