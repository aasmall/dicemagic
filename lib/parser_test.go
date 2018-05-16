package lib

import (
	"reflect"
	"strings"
	"testing"
)

func Test_populateRequired(t *testing.T) {
	type args struct {
		tok       token
		lit       string
		tokExpect token
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "Find NUMBER",
			args:    args{numberToken, "6", numberToken},
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
		tok       token
		lit       string
		tokExpect token
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{name: "Find NUMBER",
			args:  args{numberToken, "6", numberToken},
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
		want    *RollExpression
		wantErr bool
	}{
		{name: "ROLL (1d12+7)/2[mundane]+1d4[fire]",
			p: NewParser(strings.NewReader("ROLL (1d12+7)/2[mundane]+1D4[fire]")),
			want: &RollExpression{"Roll (1d12+7)/2[Mundane]+1d4[Fire]", []Segment{
				Segment{Number: 1, Operator: "+", SegmentType: "Mundane", EvaluationPriority: -2},
				Segment{Number: 12, Operator: "d", SegmentType: "Mundane", EvaluationPriority: -3},
				Segment{Number: 7, Operator: "+", SegmentType: "Mundane", EvaluationPriority: -2},
				Segment{Number: 2, Operator: "/", SegmentType: "Mundane", EvaluationPriority: -1},
				Segment{Number: 1, Operator: "+", SegmentType: "Fire", EvaluationPriority: 0},
				Segment{Number: 4, Operator: "d", SegmentType: "Fire", EvaluationPriority: -4}}},
			wantErr: false}, {name: "ROLL (1d12+7)/2 mundane +1d4 fire",
			p: NewParser(strings.NewReader("ROLL (1d12+7)/2mundane+1d4 fire")),
			want: &RollExpression{"Roll (1d12+7)/2[Mundane]+1d4[Fire]", []Segment{
				Segment{Number: 1, Operator: "+", SegmentType: "Mundane", EvaluationPriority: -2},
				Segment{Number: 12, Operator: "d", SegmentType: "Mundane", EvaluationPriority: -3},
				Segment{Number: 7, Operator: "+", SegmentType: "Mundane", EvaluationPriority: -2},
				Segment{Number: 2, Operator: "/", SegmentType: "Mundane", EvaluationPriority: -1},
				Segment{Number: 1, Operator: "+", SegmentType: "Fire", EvaluationPriority: 0},
				Segment{Number: 4, Operator: "d", SegmentType: "Fire", EvaluationPriority: -4}}},
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
