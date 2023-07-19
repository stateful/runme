package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: add more unit tests

func Test_maxEnvSize(t *testing.T) {
	{
		session := newEnvStore()

		_, err := session.Set("key", strings.Repeat("a", maxEnvSize))
		assert.Error(t, err)
	}

	{
		session := newEnvStore()

		_, err := session.Set("key", "value")
		assert.NoError(t, err)
	}

	{
		session := newEnvStore()

		{
			_, err := session.Set("key", strings.Repeat("a", maxEnvSize/2+1))
			assert.NoError(t, err)
		}

		{
			_, err := session.Set("key", strings.Repeat("a", (maxEnvSize/2)+1))
			assert.NoError(t, err)
		}
	}

	{
		session := newEnvStore()

		{
			_, err := session.Set("key", strings.Repeat("a", maxEnvSize/2+1))
			assert.NoError(t, err)
		}

		{
			_, err := session.Set("key2", strings.Repeat("a", (maxEnvSize/2)+1))
			assert.Error(t, err)
		}
	}
}
