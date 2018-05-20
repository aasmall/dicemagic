package roll

import (
	"sort"
	"testing"

	"math"

	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

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
			name:    "2d12",
			args:    args{loops: 100000, minPValue: .05, numberOfDice: 2, sides: 12},
			want:    true,
			wantErr: false},
		{
			name:    "2d6",
			args:    args{loops: 100000, minPValue: .05, numberOfDice: 2, sides: 6},
			want:    true,
			wantErr: false},
		{
			name:    "3d20",
			args:    args{loops: 100000, minPValue: .05, numberOfDice: 3, sides: 20},
			want:    true,
			wantErr: false},
		{
			name:    "8d4",
			args:    args{loops: 500000, minPValue: .05, numberOfDice: 8, sides: 4},
			want:    true,
			wantErr: false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testRoll(t, tt.args.loops, tt.args.minPValue, tt.args.numberOfDice, tt.args.sides)
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

func testRoll(t *testing.T, loops int, minPValue float64, numberOfDice int64, sides int64) (bool, error) {
	m := make(map[int64]int)
	for i := numberOfDice; i < numberOfDice*sides; i++ {
		m[i] = 0
	}
	for i := 0; i < loops; i++ {
		x, err := Roll(numberOfDice, sides)
		if err != nil {
			return false, err
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
	for _, k := range keys {
		obs = append(obs, float64(m[k]))
		prob := diceProbability(t, numberOfDice, sides, k)
		exp = append(exp, prob*float64(loops))
		t.Logf("%6d : %10.5g%% : %8.5g : %8g", k, prob*100, prob*float64(loops), float64(m[k]))
		df++
	}
	c := stat.ChiSquare(obs, exp)
	p := 1 - distuv.ChiSquared{K: float64(df), Src: nil}.CDF(c)
	t.Logf("chi2=%v, df=%v, p=%v", c, df, p)
	if p > minPValue {
		return true, nil
	}
	return false, nil
}
func diceProbability(t *testing.T, numberOfDice int64, sides int64, target int64) float64 {
	rollAmount := math.Pow(float64(sides), float64(numberOfDice))
	targetAmount := float64(0)
	var possibilities []int64
	for i := int64(1); i <= sides; i++ {
		possibilities = append(possibilities, i)
	}
	c := make(chan []int64)
	go GenerateProducts(c, possibilities, numberOfDice)
	for product := range c {
		if sumInt64(product...) == target {
			targetAmount++
		}
	}
	p := (targetAmount / rollAmount)
	return p
}

func GenerateProducts(c chan []int64, possibilities []int64, numberOfDice int64) {
	lens := int64(len(possibilities))
	for ix := make([]int64, numberOfDice); ix[0] < lens; NextIndex(ix, lens) {
		r := make([]int64, numberOfDice)
		for i, j := range ix {
			r[i] = possibilities[j]
		}
		c <- r
	}
	close(c)
}
func NextIndex(ix []int64, lens int64) {
	for j := len(ix) - 1; j >= 0; j-- {
		ix[j]++
		if j == 0 || ix[j] < lens {
			return
		}
		ix[j] = 0
	}
}
func sumInt64(nums ...int64) int64 {
	r := int64(0)
	for _, n := range nums {
		r += n
	}
	return r
}
