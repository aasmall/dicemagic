package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

//RollExpression is a collection of Segments
type RollExpression struct {
	initialText string
	Segments    []Segment
}

//Segments is half of a mathmatical expression along it's its evaluation priority
type Segment struct {
	Operator           string
	Number             int64
	SegmentType        string
	EvaluationPriority int
}

//RollTotal represents collapsed Segments that have been evaluated
type RollTotal struct {
	rollType   string
	rollResult int64
}

func GetHighestPriority(r []Segment) int {
	highestPriority := 0
	for _, e := range r {
		if e.EvaluationPriority < highestPriority {
			highestPriority = e.EvaluationPriority
		}

	}
	return highestPriority
}

func (r *RollExpression) String() string {
	return r.initialText
}

func (r *RollExpression) getTotalsByType() ([]RollTotal, error) {
	//var lastSegment Segment
	m := make(map[string]int64)
	rollTotals := []RollTotal{}
	//break segments into their Damage Types
	segmentsPerSegmentType := make(map[string][]Segment)
	for _, e := range r.Segments {
		segmentsPerSegmentType[e.SegmentType] = append(segmentsPerSegmentType[e.SegmentType], e)
	}

	//for each damage type
	for k, remainingSegments := range segmentsPerSegmentType {
		// Establish highest priority (represented as lowest number)
		highestPriority := GetHighestPriority(remainingSegments)
		var lastSegment Segment

		//loop through priorities
		for p := highestPriority; p < 1; p++ {
			for i := 0; i < len(remainingSegments); i++ {
				if !strings.ContainsAny(remainingSegments[i].Operator, "D+-*/") {
					return rollTotals, fmt.Errorf("%s is not a valid operator", remainingSegments[i].Operator)
				}
				if remainingSegments[i].EvaluationPriority == p && len(remainingSegments) > 1 && i > 0 {
					replacementSegment, err := doMath(lastSegment, remainingSegments[i])
					if err != nil {
						return rollTotals, err
					}
					remainingSegments = insertAtLocation(deleteAtLocation(remainingSegments, i-1, 2), replacementSegment, i-1)
					lastSegment = replacementSegment
					i--
				} else {
					lastSegment = remainingSegments[i]
				}
			}
		}
		//I have fully collapsed this loop. Add to final result.
		m[k] += lastSegment.Number
	}

	//sort it
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		rollTotals = append(rollTotals, RollTotal{rollType: k, rollResult: m[k]})
	}
	return rollTotals, nil
}
func roll(numberOfDice int64, sides int64) (int64, error) {
	if numberOfDice > 1000 {
		err := fmt.Errorf("I can't hold that many dice")
		return 0, err
	} else if sides > 1000 {
		err := fmt.Errorf("A die with that many sides is basically round")
		return 0, err
	} else {
		result := int64(0)
		for i := int64(0); i < numberOfDice; i++ {
			x, err := generateRandomInt(1, sides)
			if err != nil {
				return 0, err
			}
			result += x
		}
		return result, nil
	}
}

func deleteAtLocation(segment []Segment, location int, numberToDelete int) []Segment {
	return append(segment[:location], segment[location+numberToDelete:]...)
}
func insertAtLocation(segment []Segment, segmentToInsert Segment, location int) []Segment {
	segment = append(segment, segmentToInsert)
	copy(segment[location+1:], segment[location:])
	segment[location] = segmentToInsert
	return segment
}
func doMath(leftMod Segment, rightmod Segment) (Segment, error) {
	m := Segment{}
	switch rightmod.Operator {
	case "*":
		m.Number = leftMod.Number * rightmod.Number
	case "/":
		if rightmod.Number == 0 {
			return m, fmt.Errorf("Don't make me break the universe.")
		}
		m.Number = leftMod.Number / rightmod.Number
	case "+":
		m.Number = leftMod.Number + rightmod.Number
	case "-":
		m.Number = leftMod.Number - rightmod.Number
	case "D":
		num, err := roll(leftMod.Number, rightmod.Number)
		m.Number = num
		if err != nil {
			return m, err
		}
	}
	m.Operator = leftMod.Operator
	m.EvaluationPriority = leftMod.EvaluationPriority
	return m, nil
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
