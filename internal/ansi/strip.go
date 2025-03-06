package ansi

import "regexp"

const stripAnsi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var stripAnsiRegexp = regexp.MustCompile(stripAnsi)

func Strip(line []byte) []byte {
	return stripAnsiRegexp.ReplaceAll(line, []byte{})
}
