package owl

import (
	"bytes"
)

// todo(sebastian): replace with robust impl
func ParseRawSpec(single []byte) (k, v, s string, m bool) {
	v, s, m = "", "", false

	eqIdx := bytes.Index(single, []byte{'='})
	k = string(single[0:eqIdx])
	remainder := bytes.Split(single[eqIdx+1:], []byte{'#'})

	if len(remainder) > 1 {
		v = string(bytes.Trim(remainder[0], " "))
		end := bytes.Trim(remainder[1], " ")
		sParts := bytes.Split(end, []byte{'!'})
		s = string(sParts[0])
		m = len(sParts) > 1
	} else {
		v = string(remainder[0])
	}

	return k, v, s, m
}
