package dicelang

import (
	"bytes"
	"strconv"
)

func MergeDiceTotalMaps(mapsToMerge ...map[string]float64) map[string]float64 {
	retMap := make(map[string]float64)
	for _, mp := range mapsToMerge {
		for k, v := range mp {
			retMap[k] += v
		}
	}
	return retMap
}

//GetDiceSet returns the sum of an AST, a DiceSet, and an error
func (t *AST) GetDiceSet() (float64, DiceSet, error) {
	v, ret, err := t.eval(&DiceSet{})
	return v, *ret, err
}

//GetDiceSets merges all statements in ...*AST and returns a merged diceTotalMap and all rolled dice.
func GetDiceSets(stmts ...*AST) (map[string]float64, []Dice, error) {
	var maps []map[string]float64
	var dice []Dice

	for i := 0; i < len(stmts); i++ {
		_, ds, err := stmts[i].GetDiceSet()
		if err != nil {
			return nil, dice, err
		}
		maps = append(maps, ds.TotalsByColor)
		for _, d := range ds.Dice {
			dice = append(dice, d)
		}
	}
	return MergeDiceTotalMaps(maps...), dice, nil
}

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
