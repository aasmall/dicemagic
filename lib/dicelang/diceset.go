package dicelang

import (
	"fmt"
	"strconv"
)

// GetTotal sums the total of all DiceSets in ds.
func (ds *DiceSets) GetTotal() (int64, error) {
	var total float64
	for _, set := range ds.DiceSet {
		for _, v := range set.TotalsByColor {
			total = total + v
		}
	}
	return float64ToInt64(total)
}

// MergeDiceTotalMaps merges like colors in DiceSets and returns a map of colors and their total
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

// GetDiceSet returns the sum of an AST, a DiceSet, and an error
// THIS IS THE THING THAT ROLLS STUFF.
// CALL ONLY ONCE PER REQUEST
func (t *AST) GetDiceSet() (float64, *DiceSet, error) {
	ds := &DiceSet{}
	v, ret, err := t.eval(ds)
	if err != nil {
		return 0, &DiceSet{}, err
	}
	return v, ret, err
}

// func TotalsMapString(m map[string]float64) string {
// 	var b [][]byte
// 	if len(m) == 1 && m[""] != 0 {
// 		return strconv.FormatFloat(m[""], 'f', 1, 64)
// 	}
// 	for k, v := range m {
// 		if k == "" {
// 			b = append(b, []byte("Unspecified"))
// 		} else {
// 			b = append(b, []byte(k))
// 		}
// 		b = append(b, []byte(": "))
// 		b = append(b, []byte(strconv.FormatFloat(v, 'f', 1, 64)))
// 	}
// 	return string(bytes.Join(b, []byte(", ")))
// }
// func FacesSliceString(faces []int64) string {
// 	var b [][]byte
// 	for _, f := range faces {
// 		b = append(b, []byte(strconv.FormatInt(f, 10)))
// 	}
// 	return string(bytes.Join(b, []byte(", ")))
// }

func float64ToInt64(f float64) (int64, error) {
	s := fmt.Sprintf("%.0f", f)
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return i, nil
}
