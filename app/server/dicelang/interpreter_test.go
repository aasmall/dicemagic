package dicelang

import (
	"reflect"
	"sort"
	"testing"

	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

func TestAST_GetDiceSet(t *testing.T) {
	roll1d20AST, _, _ := NewParser("roll 20d1 mundane").Statements()
	tests := []struct {
		name    string
		t       *AST
		want    float64
		want1   DiceSet
		wantErr bool
	}{
		{name: "roll 1d20 mundane",
			t:    roll1d20AST[0],
			want: 20,
			want1: DiceSet{
				Dice: []Dice{
					Dice{
						Count:       20,
						Sides:       1,
						Total:       20,
						Faces:       []int64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						Max:         20,
						Min:         20,
						DropHighest: 0,
						DropLowest:  0,
						Color:       "Mundane"}},
				TotalsByColor: map[string]float64{"Mundane": float64(20)},
				dropHighest:   0,
				dropLowest:    0,
				colors:        []string{},
				colorDepth:    0},
			wantErr: false}}
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
				t.Errorf("AST.GetDiceSet() got1 = %+v, want %+v", got1, tt.want1)
			}
		})
	}
}

func Test_generateRandomInt(t *testing.T) {
	numberOfBuckets := int64(200)
	numberOfLoops := 1000000
	m := make(map[int64]int)
	for i := 0; i < numberOfLoops; i++ {
		x, _ := generateRandomInt(1, numberOfBuckets)
		m[x]++
	}
	var obs []float64
	var exp []float64
	expv := float64(int64(numberOfLoops) / numberOfBuckets)
	if len(m) != int(numberOfBuckets) {
		t.Errorf("bad distribution of random numbers")
	}
	for e := range m {
		obs = append(obs, float64(m[e]))
		exp = append(exp, expv)
	}
	c := stat.ChiSquare(obs, exp)
	p := 1 - distuv.ChiSquared{K: float64(numberOfBuckets - 1), Src: nil}.CDF(c)
	t.Logf("chi2=%v, df=%v, p=%v", c, numberOfBuckets-1, p)
}

func TestRoll(t *testing.T) {
	type args struct {
		biasMod      int64
		biasTo       int64
		biasFreq     float64
		loops        int
		minPValue    float64
		numberOfDice int64
		sides        int64
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "2d12",
			args: args{
				biasMod:      0,
				biasTo:       0,
				biasFreq:     0,
				loops:        100000,
				minPValue:    .01,
				numberOfDice: 2,
				sides:        12},
			want:    true,
			wantErr: false},
		{
			name:    "2d6",
			args:    args{biasMod: 0, biasTo: 0, biasFreq: 0, loops: 100000, minPValue: .01, numberOfDice: 2, sides: 6},
			want:    true,
			wantErr: false},
		{
			name:    "3d20",
			args:    args{biasMod: 0, biasTo: 0, biasFreq: 0, loops: 100000, minPValue: .01, numberOfDice: 3, sides: 20},
			want:    true,
			wantErr: false},
		{
			name:    "3d20 bias +1",
			args:    args{biasMod: 1, biasTo: 0, biasFreq: 0, loops: 100000, minPValue: .01, numberOfDice: 3, sides: 20},
			want:    false,
			wantErr: false},
		{
			name:    "3d20 1% bias",
			args:    args{biasMod: 0, biasTo: 31, biasFreq: .01, loops: 100000, minPValue: .01, numberOfDice: 3, sides: 20},
			want:    false,
			wantErr: false},
		{
			name:    "8d4",
			args:    args{biasMod: 0, biasTo: 0, biasFreq: 0, loops: 500000, minPValue: .01, numberOfDice: 8, sides: 4},
			want:    true,
			wantErr: false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testRoll(t, tt.args.biasMod, tt.args.biasTo, tt.args.biasFreq, tt.args.loops, tt.args.minPValue, tt.args.numberOfDice, tt.args.sides)
			if (err != nil) != tt.wantErr {
				t.Errorf("testRoll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("testRoll() = %v, want %v", got, tt.want)
			}
		})
	}
}

type rollBucket struct {
	result int64
	count  int64
}

func testRoll(t *testing.T, biasMod int64, biasTo int64, biasFreq float64, loops int, minPValue float64, numberOfDice int64, sides int64) (bool, error) {
	m := make(map[int64]int)
	for i := numberOfDice; i < numberOfDice*sides; i++ {
		m[i] = 0
	}
	biasCount := 0
	for i := 0; i < loops; i++ {
		_, x, err := roll(numberOfDice, sides, 0, 0)
		if err != nil {
			return false, err
		}
		//calculate biases
		x += biasMod
		if biasFreq > 0 {
			if i%int(1/biasFreq) == 0 {
				biasCount++
				x = biasTo
			}
		}
		m[x]++
	}

	var obs []float64
	var exp []float64
	var keys []int64
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	df := -1

	t.Logf("Rolling %dd%d %d times", numberOfDice, sides, loops)
	t.Logf("Bucket : Probability : Expected : Observed")
	t.Logf("------------------------------------------")
	probMap := DiceProbability(numberOfDice, sides, 0, 0)
	for _, k := range keys {
		obs = append(obs, float64(m[k]))
		prob := probMap[k] / 100
		exp = append(exp, prob*float64(loops))
		t.Logf("%6d : %10.5g%% : %8.5g : %8g", k, probMap[k], prob*float64(loops), float64(m[k]))
		df++
	}
	c := stat.ChiSquare(obs, exp)
	p := 1 - distuv.ChiSquared{K: float64(df), Src: nil}.CDF(c)
	t.Logf("chi2=%v, df=%v, p=%v", c, df, p)
	if biasFreq > 0 {
		t.Logf("Biased to %v %d times", biasTo, biasCount)
	}
	if p > minPValue {
		return true, nil
	}
	return false, nil
}
