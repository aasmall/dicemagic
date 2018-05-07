package internal

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aasmall/dicemagic/roll"
)

//RollExpression is a collection of Segments
type RollExpression struct {
	InitialText string    `datastore:",noindex"`
	Segments    []Segment `datastore:",noindex"`
}

//Segment is half of a mathmatical expression along it's its evaluation priority
type Segment struct {
	Operator           string `datastore:",noindex"`
	Number             int64  `datastore:",noindex"`
	SegmentType        string `datastore:",noindex"`
	EvaluationPriority int    `datastore:",noindex"`
}

//RollTotal represents collapsed Segments that have been evaluated
type RollTotal struct {
	RollType   string
	RollResult int64
}

func getHighestPriority(r []Segment) int {
	highestPriority := 0
	for _, e := range r {
		if e.EvaluationPriority < highestPriority {
			highestPriority = e.EvaluationPriority
		}

	}
	return highestPriority
}

func (r *RollExpression) String() string { return r.InitialText }

//GetTotalsByType return slices of RollTotal for a RollExpression
func (r *RollExpression) GetTotalsByType() ([]RollTotal, error) {
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
		highestPriority := getHighestPriority(remainingSegments)
		var lastSegment Segment

		//loop through priorities
		for p := highestPriority; p < 1; p++ {
			for i := 0; i < len(remainingSegments); i++ {
				if !strings.ContainsAny(remainingSegments[i].Operator, "d+-*/") {
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
		m[k] += int64(lastSegment.Number)
	}

	//sort it
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		rollTotals = append(rollTotals, RollTotal{RollType: k, RollResult: m[k]})
	}
	return rollTotals, nil
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
	case "d":
		num, err := roll.Roll(leftMod.Number, rightmod.Number)
		m.Number = num
		if err != nil {
			return m, err
		}
	}
	m.Operator = leftMod.Operator
	m.EvaluationPriority = leftMod.EvaluationPriority
	return m, nil
}
