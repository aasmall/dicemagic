package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	//"math"
	"math/big"
	"net/http"
	"strings"
)

var ctx context.Context

func parseHandler(w http.ResponseWriter, r *http.Request) {
	ctx = appengine.NewContext(r)
	//Decode request into ParseRequest type
	parseRequest := new(ParseRequest)
	json.NewDecoder(r.Body).Decode(parseRequest)

	//Prepare Response Object
	parseResponse := new(ParseResponse)

	//Call Parser and inject response into response object
	parsedString, err := parseString(parseRequest.Text)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		parseResponse.Text = err.Error()

	} else {
		parseResponse.Text = parsedString
	}
	//Encode response into response stream
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parseResponse)
}

func (a *RollStatement) totalDamage() (map[string]int64, error) {
	m := make(map[string]int64)
	err := a.RollSegments()
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
		case "*":
			if e.DamageType == "" {
				m[strings.Title(a.DiceSegments[i-1].DamageType)] *= e.DiceRollResult
				a.DiceSegments[i].DamageType = a.DiceSegments[i-1].DamageType
			} else {
				m[strings.Title(e.DamageType)] += e.DiceRollResult
			}
		case "/":
			if e.DamageType == "" {
				m[strings.Title(a.DiceSegments[i-1].DamageType)] = m[strings.Title(a.DiceSegments[i-1].DamageType)] / e.DiceRollResult
				a.DiceSegments[i].DamageType = a.DiceSegments[i-1].DamageType
			} else {
				m[strings.Title(e.DamageType)] += e.DiceRollResult
			}
		}
		trailingOperator = e.TrailingOperator
	}
	return m, nil
}
func (d *DiceSegment) roll() error {

	if d.DiceRoll.NumberOfDice > 1000 {
		err := fmt.Errorf("I can't hold that many dice.")
		return err
	} else if d.DiceRoll.Sides > 1000 {
		err := fmt.Errorf("A die with that many sides is basically round")
		return err
	} else {
		result := int64(0)
		for i := int64(0); i < d.DiceRoll.NumberOfDice; i++ {
			x, err := GenerateRandomInt64(1, d.DiceRoll.Sides)
			if err != nil {
				return err
			}
			result += x
		}

		//manage Modifiers
		if d.Modifier > 1<<31-1 {
			err := fmt.Errorf("I roll dice, I'm not a calculator.")
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
func (d *RollStatement) RollSegments() error {
	for i, _ := range d.DiceSegments {
		err := d.DiceSegments[i].roll()
		if err != nil {
			return err
		}
	}
	return nil
}

func GenerateRandomInt64(min int64, max int64) (int64, error) {
	size := max - min + 1
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(size)))
	if err != nil {
		err = fmt.Errorf("Couldn't make a random number. Out of entropy?")
		return 0, err
	}
	n := nBig.Int64()
	return n + int64(min), nil
}

func parseString(text string) (string, error) {
	stmt, err := NewParser(strings.NewReader(text)).Parse()
	if err != nil {
		return "", err
	}
	log.Debugf(ctx, "stmt: %#v", stmt)
	totalDamageString, err := stmt.totalDamage()
	return fmt.Sprintf("%+v ", totalDamageString), err
}
