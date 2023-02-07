package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func mustTimeParse(t time.Time, err error) time.Time {
	if err != nil {
		panic(err)
	}
	return t
}

func TestDateTime(t *testing.T) {
	testCases := []struct {
		in       string
		expected time.Time
	}{
		{
			// Describes Stateful GraphQL API's scalar "DataTime" format.
			// TODO(adamb): it's unclear in what timezone this date really is.
			in:       "2021-12-28T07:22:37.887000+00:00",
			expected: mustTimeParse(time.Parse(time.RFC3339, "2021-12-28T07:22:37.887000+00:00")),
		},
		{
			// Describes Stateful GraphQL API's scalar "Time" format.
			// TODO(adamb): it's unclear in what timezone this date really is.
			in:       "2021-12-29T10:00:00+01:00",
			expected: mustTimeParse(time.Parse(time.RFC3339, "2021-12-29T10:00:00+01:00")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			d := DateTime{}
			err := d.UnmarshalJSON([]byte(fmt.Sprintf(`"%s"`, tc.in)))
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, time.Time(d))
		})
	}
}
