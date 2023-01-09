package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_dropANSIEscape(t *testing.T) {
	result := dropANSIEscape([]byte("\u001b[?2004hbash-5.2$"))
	assert.Equal(t, "bash-5.2$", string(result))
}
