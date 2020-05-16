package dicelang

import (
	"bytes"
	"fmt"
	"strconv"
)

func DiceSetsFromSlice(s []*DiceSet) DiceSets {
	return DiceSets{DiceSet: s}
}

func (ds *DiceSets) GetTotal() (int64, error) {
	var total float64
	for _, set := range ds.DiceSet {
		for _, v := range set.TotalsByColor {
			total = total + v
		}
	}
	return float64ToInt64(total)
}

func float64ToInt64(f float64) (int64, error) {
	s := fmt.Sprintf("%.0f", f)
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return i, nil
}

func (ds *DiceSets) MergeDiceTotalMaps() map[string]float64 {
	var maps []map[string]float64
	for _, set := range ds.DiceSet {
		maps = append(maps, set.TotalsByColor)
	}
	returnMap := make(map[string]float64)
	for _, mergeMap := range maps {
		for k, v := range mergeMap {
			returnMap[k] += v
		}
	}
	return returnMap
}

//GetDiceSet returns the sum of an AST, a DiceSet, and an error
//THIS IS THE THING THAT ROLLS STUFF
func (t *AST) GetDiceSet() (float64, *DiceSet, error) {
	ds := &DiceSet{}
	v, ret, err := t.eval(ds)
	if err != nil {
		return 0, &DiceSet{}, err
	}
	return v, ret, err
}

//GetDiceSets merges all statements in ...*AST and returns a merged diceTotalMap and all rolled dice.
// func GetDiceSets(stmts ...*AST) (map[string]float64, []Dice, error) {
// 	var maps []map[string]float64
// 	var dice []Dice

// 	for i := 0; i < len(stmts); i++ {
// 		_, ds, err := stmts[i].GetDiceSet()
// 		if err != nil {
// 			return nil, dice, err
// 		}
// 		maps = append(maps, ds.TotalsByColor)
// 		for _, d := range ds.Dice {
// 			dice = append(dice, d)
// 		}
// 	}
// 	return MergeDiceTotalMaps(maps...), dice, nil
// }

func TotalsMapString(m map[string]float64) string {
	var b [][]byte
	if len(m) == 1 && m[""] != 0 {
		return strconv.FormatFloat(m[""], 'f', 1, 64)
	}
	for k, v := range m {
		if k == "" {
			b = append(b, []byte("Unspecified"))
		} else {
			b = append(b, []byte(k))
		}
		b = append(b, []byte(": "))
		b = append(b, []byte(strconv.FormatFloat(v, 'f', 1, 64)))
	}
	return string(bytes.Join(b, []byte(", ")))
}
func FacesSliceString(faces []int64) string {
	var b [][]byte
	for _, f := range faces {
		b = append(b, []byte(strconv.FormatInt(f, 10)))
	}
	return string(bytes.Join(b, []byte(", ")))
}
