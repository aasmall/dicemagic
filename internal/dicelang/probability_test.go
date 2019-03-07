package dicelang

import (
	"testing"
)

var result map[int64]float64

func benchmarkDiceProbability(numberOfDice, sides, H, L int64, b *testing.B) {
	var r map[int64]float64
	for n := 0; n < b.N; n++ {
		r = DiceProbability(numberOfDice, sides, H, L)
	}
	result = r
}
func BenchmarkDiceProbability1(b *testing.B)        { benchmarkDiceProbability(2, 6, 0, 0, b) }
func BenchmarkDiceProbability4d6Lx10(b *testing.B)  { benchmarkDiceProbability(4, 6, 0, 1, b) }
func BenchmarkDiceProbability20d20x10(b *testing.B) { benchmarkDiceProbability(20, 20, 0, 0, b) }

//https://anydice.com/ used for "correct" values
func TestDiceProbability(t *testing.T) {
	type args struct {
		numberOfDice int64
		sides        int64
		H            int64
		L            int64
	}
	tests := []struct {
		name string
		args args
		want map[int64]float64
	}{
		{name: "4d6-L",
			args: args{numberOfDice: 4, sides: 6, H: 0, L: 1},
			want: map[int64]float64{
				3:  0.077160494,
				4:  0.308641975,
				5:  0.771604938,
				6:  1.620370370,
				7:  2.932098765,
				8:  4.783950617,
				9:  7.021604938,
				10: 9.413580247,
				11: 11.419753086,
				12: 12.885802469,
				13: 13.271604938,
				14: 12.345679012,
				15: 10.108024691,
				16: 7.253086420,
				17: 4.166666667,
				18: 1.620370370}},
		{name: "3d20-H2",
			args: args{numberOfDice: 3, sides: 20, H: 2, L: 0},
			want: map[int64]float64{
				1:  14.2625,
				2:  12.8375,
				3:  11.4875,
				4:  10.2125,
				5:  9.0125,
				6:  7.8875,
				7:  6.8375,
				8:  5.8625,
				9:  4.9625,
				10: 4.1375,
				11: 3.3875,
				12: 2.7125,
				13: 2.1125,
				14: 1.5875,
				15: 1.1375,
				16: 0.7625,
				17: 0.4625,
				18: 0.2375,
				19: 0.0875,
				20: 0.0125}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DiceProbability(tt.args.numberOfDice, tt.args.sides, tt.args.H, tt.args.L); !deepEqualFloatMap(got, tt.want) {
				t.Errorf("DiceProbability() = %v, want %v", got, tt.want)
			}
		})
	}
}

func deepEqualFloatMap(left, right map[int64]float64) bool {
	if len(left) != len(right) {
		return false
	}
	for k, v := range left {
		if !floatEquals(v, right[k]) {
			return false
		}
	}
	return true
}
func floatEquals(a, b float64) bool {
	espilon := float64(0.00000001)
	if (a-b) < espilon && (b-a) < espilon {
		return true
	}
	return false
}
