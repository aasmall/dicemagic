package api

import (
	"reflect"
	"strings"
	"testing"
)

func TestRollExpression_getTotalsByType(t *testing.T) {
	tests := []struct {
		name    string
		r       *RollExpression
		want    []RollTotal
		wantErr bool
	}{
		{name: "fobar",
			r: NewParser(strings.NewReader("ROLL (21d1+7)/2[mundane]+4d1[fire]")).MustParse(),
			want: []RollTotal{
				{rollType: "Fire", rollResult: 4},
				{rollType: "Mundane", rollResult: 14}},
			wantErr: false},
		{name: "fobar",
			r: NewParser(strings.NewReader("ROLL (8d1+10)*2+5[mundane]+6d1/2[fire]")).MustParse(),
			want: []RollTotal{
				{rollType: "Fire", rollResult: 3},
				{rollType: "Mundane", rollResult: 41}},
			wantErr: false},
		{name: "No Types",
			r: NewParser(strings.NewReader("ROLL (8d1+10)*2+5")).MustParse(),
			want: []RollTotal{
				{rollType: "", rollResult: 41}},
			wantErr: false},
		{name: "mixmatched types",
			r: NewParser(strings.NewReader("ROLL (8d1+10)*2+5[mundane]+6d1/2*5-10")).MustParse(),
			want: []RollTotal{
				{rollType: "", rollResult: 5},
				{rollType: "Mundane", rollResult: 41}},
			wantErr: false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.r.getTotalsByType()
			if (err != nil) != tt.wantErr {
				t.Errorf("RollExpression.getTotalsByType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RollExpression.getTotalsByType() = %v, want %v", got, tt.want)
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
			r:    NewParser(strings.NewReader("ROLL (21d1+7)/2[mundane]+4d1[fire]")).MustParse(),
			want: "Roll (21D1+7)/2[Mundane]+4D1[Fire]"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.String(); got != tt.want {
				t.Errorf("RollExpression.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
