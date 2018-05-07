package roll

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

//Roll creates a random number that represents the roll of
//some dice
func Roll(numberOfDice int64, sides int64) (int64, error) {
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
	size := max - min + 1
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(size)))
	if err != nil {
		err = fmt.Errorf("Couldn't make a random number. Out of entropy?")
		return 0, err
	}
	n := nBig.Int64()
	return n + int64(min), nil
}
