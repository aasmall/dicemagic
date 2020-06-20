package dicelang

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// StringFromRollResponse parses the response from the dicelang-server into a human readable format
func (rr *RollResponse) StringFromRollResponse() string {
	var s []string
	var finalTotal int64
	for _, ds := range rr.DiceSets {
		var faces []interface{}
		for _, d := range ds.Dice {
			faces = append(faces, facesSliceString(d.Faces))
		}
		s = append(s, fmt.Sprintf("%s = *%s*", fmt.Sprintf(ds.ReString, faces...), strconv.FormatInt(ds.Total, 10)))
		finalTotal = finalTotal + ds.Total
	}
	if len(rr.DiceSets) > 1 {
		s = append(s, fmt.Sprintf("Total: %s", strconv.FormatInt(finalTotal, 10)))
	}
	return strings.Join(s, "\n")
}

func facesSliceString(faces []int64) string {
	var b [][]byte
	for _, f := range faces {
		b = append(b, []byte(strconv.FormatInt(f, 10)))
	}
	return string(bytes.Join(b, []byte(", ")))
}
