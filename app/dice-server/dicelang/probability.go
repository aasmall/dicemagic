package dicelang

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"math/big"
)

//DiceProbability returns a map of results to probabilities (in percent) for a given roll of dice
//numberOfDice (int64): the number of dice in the throw.
//sides (int64): the sides each die has
//H: The number of high dice to drop (set to 0 to not drop any)
//L: The number of low dice to drop (set to 0 to not drop any)
//credit to https://stackoverflow.com/questions/50690348/calculate-probability-of-a-fair-dice-roll-in-non-exponential-time
func DiceProbability(numberOfDice, sides, H, L int64) map[int64]float64 {
	mw := newMemoWrap()
	d := mw.outcomes(numberOfDice, sides, H, L)
	var sum, denominator float64
	for _, v := range d {
		sum += v
	}
	denominator = sum / 100
	for k, v := range d {
		d[k] = v / denominator
	}
	return d
}

type memoWrap struct {
	hasher hash.Hash
	cache  map[string]map[int64]float64
}

func (mw *memoWrap) Save(args []int64, value map[int64]float64) {
	b := make([]byte, 8)
	for _, v := range args {
		binary.LittleEndian.PutUint64(b, uint64(v))
		mw.hasher.Write(b)
	}
	mw.cache[hex.EncodeToString(mw.hasher.Sum(nil))] = value
	mw.hasher.Reset()
}
func (mw *memoWrap) Get(args []int64) map[int64]float64 {
	b := make([]byte, 8)
	for _, v := range args {
		binary.LittleEndian.PutUint64(b, uint64(v))
		mw.hasher.Write(b)
	}
	val := mw.cache[hex.EncodeToString(mw.hasher.Sum(nil))]
	mw.hasher.Reset()
	return val
}

func newMemoWrap() *memoWrap {
	mw := new(memoWrap)
	mw.hasher = md5.New()
	mw.cache = make(map[string]map[int64]float64)
	return mw
}

func (mw *memoWrap) outcomes(count, sides, dropHighest, dropLowest int64) map[int64]float64 {
	args := []int64{count, sides, dropHighest, dropLowest}
	if val := mw.Get(args); val != nil {
		return val
	}
	d, d1 := make(map[int64]float64), make(map[int64]float64)
	if count == 0 {
		d[0] = 1
	} else if sides != 0 {
		for countShowingMax := int64(0); countShowingMax <= count; countShowingMax++ {
			d1 = mw.outcomes(
				count-countShowingMax,
				sides-1,
				max(dropHighest-countShowingMax, 0),
				dropLowest)
			countShowingMaxNotDropped := max(min(countShowingMax-dropHighest, count-dropHighest-dropLowest), 0)
			sumShowingMax := countShowingMaxNotDropped * sides
			multiplier := float64(new(big.Int).Binomial(count, countShowingMax).Int64())
			for k, v := range d1 {
				oldValue := d[sumShowingMax+k]
				d[sumShowingMax+k] = oldValue + (multiplier * v)
			}
		}
	}
	mw.Save(args, d)
	return d
}

func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

func max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}
