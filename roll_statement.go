package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"google.golang.org/appengine/log"
)

//RollStatement is a collection of DiceSegments
type RollStatement struct {
	DiceSegments []DiceSegment
}

//DiceSegment represents an individual dice roll and associated attributes
type DiceSegment struct {
	DiceRoll struct {
		NumberOfDice int64
		Sides        int64
	}
	DiceRollResult   int64
	ModifierOperator string
	Modifier         int64
	DamageType       string
	TrailingOperator string
}

func (d *DiceSegment) roll() error {
	if d.DiceRoll.NumberOfDice > 1000 {
		err := fmt.Errorf("I can't hold that many dice")
		return err
	} else if d.DiceRoll.Sides > 1000 {
		err := fmt.Errorf("A die with that many sides is basically round")
		return err
	} else {
		result := int64(0)
		for i := int64(0); i < d.DiceRoll.NumberOfDice; i++ {
			x, err := generateRandomInt(1, d.DiceRoll.Sides)
			if err != nil {
				return err
			}
			result += x
		}

		//manage Modifiers
		if d.Modifier > 1<<31-1 {
			err := fmt.Errorf("I roll dice, I'm not a calculator")
			return err
		}
		switch d.ModifierOperator {
		case "+":
			result = result + d.Modifier
		case "-":
			result = result - d.Modifier
		case "*":
			result = result * d.Modifier
		case "/":
			if d.Modifier != 0 {
				result = result / d.Modifier
			} else {
				return fmt.Errorf("Don't make me break the universe. (div/0 error)")
			}
		}

		d.DiceRollResult = result

	}
	return nil
}

//rollSegments calculates rool results for all DiceSegments
//in a RollStatement
func (a *RollStatement) rollSegments() error {
	for i := range a.DiceSegments {
		err := a.DiceSegments[i].roll()
		if err != nil {
			return err
		}
	}
	return nil
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

func parseString(ctx context.Context, text string) (string, error) {
	stmt, err := NewParser(strings.NewReader(text)).Parse()
	if err != nil {
		return "", err
	}
	log.Debugf(ctx, "stmt: %#v", stmt)
	TotalDamageString, err := stmt.TotalDamage()
	return fmt.Sprintf("%+v ", TotalDamageString), err
}

//HasDamageTypes returns true if any DiceSegment has a damage type
//Useful for deciding how to parse into a string.
func (a *RollStatement) HasDamageTypes() bool {
	for _, e := range a.DiceSegments {
		if e.DamageType != "" {
			return true
		}
	}
	return false
}

//TotalDamage rolls all DiceSegments, populates the DiceRollResult and, rolls all the segments and maps damage into types
func (a *RollStatement) TotalDamage() (map[string]int64, error) {
	m := make(map[string]int64)
	err := a.rollSegments()
	if err != nil {
		return m, err
	}
	trailingOperator := ""
	for i, e := range a.DiceSegments {
		switch trailingOperator {
		case "":
			m[strings.Title(e.DamageType)] += e.DiceRollResult
		case "+":
			if e.DamageType == "" {
				m[strings.Title(a.DiceSegments[i-1].DamageType)] += e.DiceRollResult
				a.DiceSegments[i].DamageType = a.DiceSegments[i-1].DamageType
			} else {
				m[strings.Title(e.DamageType)] += e.DiceRollResult
			}
		case "-":
			if e.DamageType == "" {
				m[strings.Title(a.DiceSegments[i-1].DamageType)] -= e.DiceRollResult
				a.DiceSegments[i].DamageType = a.DiceSegments[i-1].DamageType
			} else {
				m[strings.Title(e.DamageType)] += e.DiceRollResult
			}
		}
		trailingOperator = e.TrailingOperator
	}
	return m, nil
}
func (a *RollStatement) TotalDamageString() (string, error) {
	m, err := a.TotalDamage()
	if err != nil {
		return "", err
	}
	var damageString string
	var total int64
	for k := range m {
		if k == "" {
			damageString += fmt.Sprintf("%d damage\n", m[k])
		} else {
			damageString += fmt.Sprintf("%d %s damage\n", m[k], k)
		}
		total += m[k]
	}
	damageString += fmt.Sprintf(" for a total of %d ", total)
	return damageString, nil
}
func (a *RollStatement) TotalSimpleRollString() (string, error) {
	m, err := a.TotalDamage()
	if err != nil {
		return "", err
	}
	var damageString string
	var total int64
	if len(m) == 1 {
		for k := range m {
			damageString = fmt.Sprintf("%d", m[k])
		}
	} else {
		i := 0
		for k := range m {
			if i == 0 {
				damageString += fmt.Sprintf("%d", m[k])
			} else {
				damageString += fmt.Sprintf(" and %d", m[k])
			}
			total += m[k]
			i++
		}
		damageString += fmt.Sprintf(" for a total of %d ", total)
	}
	return damageString, nil
}
