package roll

import (
	"strings"
	"testing"
)

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

func TestRollExpression_Total(t *testing.T) {
	tests := []struct {
		name    string
		r       *RollExpression
		wantErr bool
	}{
		{name: "simple",
			r:       NewParser(strings.NewReader("Roll 1d12+1d8")).MustParse(),
			wantErr: false},
		{name: "complex",
			r:       NewParser(strings.NewReader("Roll (1d12+7)/2[Mundane]+1d4[Fire]")).MustParse(),
			wantErr: false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.r.Total(); (err != nil) != tt.wantErr {
				t.Errorf("RollExpression.Total() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
