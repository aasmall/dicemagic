package main

import (
	"reflect"
	"strings"
	"testing"
)

func Test_populateRequired(t *testing.T) {
	type args struct {
		tok       Token
		lit       string
		tokExpect Token
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "Find D",
			args:    args{D, "d", D},
			want:    "d",
			wantErr: false},
		{name: "Find D Negative",
			args:    args{NUMBER, "6", D},
			want:    "",
			wantErr: true},
		{name: "Find NUMBER",
			args:    args{NUMBER, "6", NUMBER},
			want:    "6",
			wantErr: false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := populateRequired(tt.args.tok, tt.args.lit, tt.args.tokExpect)
			if (err != nil) != tt.wantErr {
				t.Errorf("populateRequired() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("populateRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_populateOptional(t *testing.T) {
	type args struct {
		tok       Token
		lit       string
		tokExpect Token
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{name: "Find NUMBER",
			args:  args{NUMBER, "6", NUMBER},
			want:  "6",
			want1: true}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := populateOptional(tt.args.tok, tt.args.lit, tt.args.tokExpect)
			if got != tt.want {
				t.Errorf("populateOptional() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("populateOptional() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		p       *Parser
		want    *RollStatement
		wantErr bool
	}{
		{name: "ROLL 1d12",
			p: NewParser(strings.NewReader("ROLL 1d12")),
			want: &RollStatement{[]DiceSegment{DiceSegment{
				DiceRoll: struct {
					NumberOfDice int64
					Sides        int64
				}{NumberOfDice: 1, Sides: 12},
				DiceRollResult:   0,
				ModifierOperator: "+",
				Modifier:         0,
				DamageType:       "",
				TrailingOperator: ""}}},
			wantErr: false},
		{name: "Roll with Modifier",
			p: NewParser(strings.NewReader("ROLL 1D12+7")),
			want: &RollStatement{[]DiceSegment{DiceSegment{
				DiceRoll: struct {
					NumberOfDice int64
					Sides        int64
				}{NumberOfDice: 1, Sides: 12},
				DiceRollResult:   0,
				ModifierOperator: "+",
				Modifier:         7,
				DamageType:       "",
				TrailingOperator: ""}}},
			wantErr: false},
		{name: "Roll with DamageType",
			p: NewParser(strings.NewReader("ROLL 2D4+7(mundane)")),
			want: &RollStatement{[]DiceSegment{DiceSegment{
				DiceRoll: struct {
					NumberOfDice int64
					Sides        int64
				}{NumberOfDice: 2, Sides: 4},
				DiceRollResult:   0,
				ModifierOperator: "+",
				Modifier:         7,
				DamageType:       "mundane",
				TrailingOperator: ""}}},
			wantErr: false},
		{name: "Roll with Division",
			p: NewParser(strings.NewReader("ROLL 2D4/2(mundane)")),
			want: &RollStatement{[]DiceSegment{DiceSegment{
				DiceRoll: struct {
					NumberOfDice int64
					Sides        int64
				}{NumberOfDice: 2, Sides: 4},
				DiceRollResult:   0,
				ModifierOperator: "/",
				Modifier:         2,
				DamageType:       "mundane",
				TrailingOperator: ""}}},
			wantErr: false},
		{name: "Roll with 2 Segments",
			p: NewParser(strings.NewReader("ROLL 2D4+7(mundane)+1d8+1(fire)")),
			want: &RollStatement{[]DiceSegment{DiceSegment{
				DiceRoll: struct {
					NumberOfDice int64
					Sides        int64
				}{NumberOfDice: 2, Sides: 4},
				DiceRollResult:   0,
				ModifierOperator: "+",
				Modifier:         7,
				DamageType:       "mundane",
				TrailingOperator: "+"}, {
				DiceRoll: struct {
					NumberOfDice int64
					Sides        int64
				}{NumberOfDice: 1, Sides: 8},
				DiceRollResult:   0,
				ModifierOperator: "+",
				Modifier:         1,
				DamageType:       "fire",
				TrailingOperator: ""}}},
			wantErr: false},
		{name: "Roll with 2 Segments and subtraction",
			p: NewParser(strings.NewReader("ROLL 2D4+7(mundane)-1d8-1(fire)")),
			want: &RollStatement{[]DiceSegment{DiceSegment{
				DiceRoll: struct {
					NumberOfDice int64
					Sides        int64
				}{NumberOfDice: 2, Sides: 4},
				DiceRollResult:   0,
				ModifierOperator: "+",
				Modifier:         7,
				DamageType:       "mundane",
				TrailingOperator: "-"}, {
				DiceRoll: struct {
					NumberOfDice int64
					Sides        int64
				}{NumberOfDice: 1, Sides: 8},
				DiceRollResult:   0,
				ModifierOperator: "-",
				Modifier:         1,
				DamageType:       "fire",
				TrailingOperator: ""}}},
			wantErr: false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.p.Parse()
			if (err != nil) != tt.wantErr {
				t.Errorf("Parser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parser.Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
