package kernel

import "regexp"

var ansiEscape = regexp.MustCompile("\u001b(?:[@-Z\\-_]|\\[[0-?]*[ -/]*[@-~])")

func dropANSIEscape(line []byte) []byte {
	return ansiEscape.ReplaceAll(line, nil)
}
