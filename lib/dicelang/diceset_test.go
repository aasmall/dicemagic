package dicelang

import (
	"reflect"
	"testing"
)

func TestAST_GetDiceSet(t *testing.T) {
	tests := []struct {
		name    string
		t       *AST
		want    float64
		want1   *DiceSet
		wantErr bool
	}{
		{
			name: "roll 1d20 red",
			t:    NewParser("ROLL 20d1 red").testStatements(),
			want: 20,
			want1: &DiceSet{
				Dice: []*Dice{
					{
						Count:       20,
						Sides:       1,
						Total:       20,
						Faces:       []int64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						Max:         20,
						Min:         20,
						DropHighest: 0,
						DropLowest:  0,
						Color:       "Red"}},
				TotalsByColor: map[string]float64{"Red": float64(20)},
				DropHighest:   0,
				DropLowest:    0,
				Colors:        []string{},
				ColorDepth:    0},
			wantErr: false},
		{
			name: "20d1 red + 12d1 blue",
			// cannot sum different colors
			t:       NewParser("20d1 red + 12d1 blue").testStatements(),
			want:    0,
			want1:   &DiceSet{},
			wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := tt.t.GetDiceSet()
			if (err != nil) != tt.wantErr {
				t.Errorf("AST.GetDiceSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("AST.GetDiceSet() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("AST.GetDiceSet() got = %+v, want %+v", got1, tt.want1)
			}
		})
	}
}
