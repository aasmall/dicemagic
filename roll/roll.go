package roll

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
)

type Dice struct {
	NumberOfDice int64
	Sides        int64
	result       int64
	Max          int64
	Min          int64
}
type DiceSet struct {
	Dice     []Dice
	Results  []int64
	DiceType string
	Min      int64
	Max      int64
}

func (d *DiceSet) Roll() ([]int64, error) {
	for _, ds := range d.Dice {
		result, err := ds.Roll()
		if err != nil {
			return nil, err
		}
		ds.Min += ds.NumberOfDice
		ds.Max += ds.NumberOfDice * ds.Sides
		d.Results = append(d.Results, result)
	}
	return d.Results, nil
}
func (d *Dice) Roll() (int64, error) {
	if d.result != 0 {
		return d.result, nil
	}
	result, err := roll(d.NumberOfDice, d.Sides)
	if err != nil {
		return 0, err
	}
	d.Min = d.NumberOfDice
	d.Max = d.NumberOfDice * d.Sides
	d.result = result
	return result, nil
}
func (d *Dice) Probability() float64 {
	if d.result > 0 {
		return diceProbability(d.NumberOfDice, d.Sides, d.result)
	}
	return 0
}

//Roll creates a random number that represents the roll of
//some dice
func roll(numberOfDice int64, sides int64) (int64, error) {
	if numberOfDice > 1000 {
		err := fmt.Errorf("I can't hold that many dice")
		return 0, err
	} else if sides > 1000 {
		err := fmt.Errorf("A die with that many sides is basically round")
		return 0, err
	} else if sides < 1 {
		err := fmt.Errorf("/me ponders the meaning of a zero sided die")
		return 0, err
	} else {
		result := int64(0)
		for i := int64(0); i < numberOfDice; i++ {
			x, err := generateRandomInt(1, int64(sides))
			if err != nil {
				return 0, err
			}
			result += x
		}
		return result, nil
	}
}

func generateRandomInt(min int64, max int64) (int64, error) {
	if max <= 0 || min < 0 {
		err := fmt.Errorf("Cannot make a random int of size zero")
		return 0, err
	}
	size := max - min
	if size == 0 {
		return 1, nil
	}
	//rand.Int does not return the max value, add 1
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(size+1)))
	if err != nil {
		err = fmt.Errorf("Couldn't make a random number. Out of entropy?")
		return 0, err
	}
	n := nBig.Int64()
	return n + int64(min), nil
}

func diceProbability(numberOfDice int64, sides int64, target int64) float64 {
	rollAmount := math.Pow(float64(sides), float64(numberOfDice))
	targetAmount := float64(0)
	var possibilities []int64
	for i := int64(1); i <= sides; i++ {
		possibilities = append(possibilities, i)
	}
	c := make(chan []int64)
	go generateProducts(c, possibilities, numberOfDice)
	for product := range c {
		if sumInt64(product...) == target {
			targetAmount++
		}
	}
	p := (targetAmount / rollAmount)
	return p
}

func generateProducts(c chan []int64, possibilities []int64, numberOfDice int64) {
	lens := int64(len(possibilities))
	for ix := make([]int64, numberOfDice); ix[0] < lens; nextIndex(ix, lens) {
		r := make([]int64, numberOfDice)
		for i, j := range ix {
			r[i] = possibilities[j]
		}
		c <- r
	}
	close(c)
}
func nextIndex(ix []int64, lens int64) {
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
