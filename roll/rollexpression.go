package roll

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//RollExpression is a collection of Segments
type RollExpression struct {
	InitialText          string        `datastore:",noindex"`
	ExpandedTextTemplate string        `datastore:",noindex"`
	DiceSet              DiceSet       `datastore:",noindex"`
	SegmentHalfs         []SegmentHalf `datastore:",noindex"`
	RollTotals           []Total       `datastore:",noindex"`
}

//Segment is a mathmatical expression
type Segment struct {
	leftSegment        SegmentHalf
	rightSegment       SegmentHalf
	SegmentType        string `datastore:",noindex"`
	EvaluationPriority int    `datastore:",noindex"`
}

//SegmentHalf is half of a mathmatical expression along it's its evaluation priority
type SegmentHalf struct {
	Operator           string `datastore:",noindex"`
	Number             int64  `datastore:",noindex"`
	SegmentType        string `datastore:",noindex"`
	EvaluationPriority int    `datastore:",noindex"`
}

//Total represents collapsed Segments that have been evaluated
type Total struct {
	RollType   string
	RollResult int64
}

func getHighestPriority(r []SegmentHalf) int {
	highestPriority := 0
	for _, e := range r {
		if e.EvaluationPriority < highestPriority {
			highestPriority = e.EvaluationPriority
		}

	}
	return highestPriority
}

func (r *RollExpression) String() string { return r.InitialText }
func (r *RollExpression) TotalsString() string {
	var buff bytes.Buffer
	rollTotal := int64(0)
	allUnspecified := true
	for _, t := range r.RollTotals {
		if t.RollType != "" {
			allUnspecified = false
		}
		rollTotal += t.RollResult
	}
	if allUnspecified {
		buff.WriteString("You rolled: ")
		buff.WriteString(strconv.FormatInt(rollTotal, 10))
	} else {
		for i, t := range r.RollTotals {
			buff.WriteString(strconv.FormatInt(t.RollResult, 10))
			buff.WriteString(" [")
			if t.RollType == "" {
				buff.WriteString("_Unspecified_")
			} else {
				buff.WriteString(t.RollType)
			}
			buff.WriteString("]")
			if i != len(r.RollTotals) {
				buff.WriteString("\n")
			} else {
				buff.WriteString("\nFor a total of: ")
				buff.WriteString(strconv.FormatInt(rollTotal, 10))
			}
		}
	}
	return buff.String()
}

//Total rolled all the dice and populates RollTotals and ExpandedText
func (r *RollExpression) Total() error {
	m := make(map[string]int64)
	rollTotals := []Total{}
	//break segments into their Damage Types
	segmentsPerSegmentType := make(map[string][]SegmentHalf)
	for _, e := range r.SegmentHalfs {
		segmentsPerSegmentType[e.SegmentType] = append(segmentsPerSegmentType[e.SegmentType], e)
	}
	//for each damage type
	for k, remainingSegments := range segmentsPerSegmentType {
		// Establish highest priority (represented as lowest number)
		highestPriority := getHighestPriority(remainingSegments)
		var lastSegment SegmentHalf

		//loop through priorities
		for p := highestPriority; p < 1; p++ {
			for i := 0; i < len(remainingSegments); i++ {
				if !strings.ContainsAny(remainingSegments[i].Operator, "d+-*/") {
					return fmt.Errorf("%s is not a valid operator", remainingSegments[i].Operator)
				}
				if remainingSegments[i].EvaluationPriority == p && len(remainingSegments) > 1 && i > 0 {
					var replacementSegment SegmentHalf
					if remainingSegments[i].Operator == "d" {
						d := Dice{NumberOfDice: lastSegment.Number, Sides: remainingSegments[i].Number}
						result, err := d.Roll()
						if err != nil {
							return err
						}
						r.DiceSet.Dice = append(r.DiceSet.Dice, d)
						replacementSegment = SegmentHalf{Operator: lastSegment.Operator, EvaluationPriority: lastSegment.EvaluationPriority, Number: result}
					} else {
						var err error
						replacementSegment, err = doMath(lastSegment, remainingSegments[i])
						if err != nil {
							return err
						}
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
		rollTotals = append(rollTotals, Total{RollType: k, RollResult: m[k]})
	}
	r.DiceSet.Roll()
	r.RollTotals = rollTotals
	return nil
}

func deleteAtLocation(segment []SegmentHalf, location int, numberToDelete int) []SegmentHalf {
	return append(segment[:location], segment[location+numberToDelete:]...)
}
func insertAtLocation(segment []SegmentHalf, segmentToInsert SegmentHalf, location int) []SegmentHalf {
	segment = append(segment, segmentToInsert)
	copy(segment[location+1:], segment[location:])
	segment[location] = segmentToInsert
	return segment
}

func doMath(leftMod SegmentHalf, rightmod SegmentHalf) (SegmentHalf, error) {
	m := SegmentHalf{}
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
