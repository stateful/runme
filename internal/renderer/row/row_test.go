package row

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewModel(t *testing.T) {
	t.Parallel()

	const (
		width              = 30
		colsCount          = 3
		charsCountOverflow = 13
	)

	testCases := []struct {
		Name     string
		Data     []string
		Expected string
	}{
		{
			Name:     "No overflow",
			Data:     []string{"a", "b", "c"},
			Expected: "a         b         c         ",
		},
		{
			Name: "No overflow full",
			Data: []string{
				strings.Repeat("a", width/colsCount),
				strings.Repeat("b", width/colsCount),
				strings.Repeat("c", width/colsCount),
			},
			Expected: `aaaaaaaaaabbbbbbbbbbcccccccccc`,
		},
		{
			Name: "All cols overflow",
			Data: []string{
				strings.Repeat("a", charsCountOverflow),
				strings.Repeat("b", charsCountOverflow),
				strings.Repeat("c", charsCountOverflow),
			},
			Expected: strings.Trim(`
aaaaaaaaaabbbbbbbbbbcccccccccc
aaa       bbb       ccc       `, "\n"),
		},
		{
			Name: "Last col overflows",
			Data: []string{
				"aaa",
				"bbb",
				strings.Repeat("c", charsCountOverflow),
			},
			Expected: strings.Trim(`
aaa       bbb       cccccccccc
                    ccc       `, "\n"),
		},
		{
			Name: "Mid col overflows",
			Data: []string{"aaa", strings.Repeat("b", charsCountOverflow), "ccc"},
			// In theory, it should fill the C column with spaces too.
			Expected: strings.Trim(`
aaa       bbbbbbbbbbccc       `+`
          bbb                 `, "\n"),
		},
		{
			Name: "Data with new lines",
			Data: []string{"aaa\naaa\n\naaa", "bbb\n\nbbb", strings.Repeat("c", charsCountOverflow)},
			Expected: strings.Trim(`
aaa       bbb       cccccccccc
aaa                 ccc       `+`
          bbb                 `+`
aaa                           `, "\n"),
		},
	}

	def := NewDefinition(width, WithPctColumns([]float64{1 / float64(colsCount), 1 / float64(colsCount), 1 / float64(colsCount)}))

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			assert.Equal(t, def.ColWidths(), []int{width / colsCount, width / colsCount, width / colsCount})
			assert.Equal(t, tc.Expected, String(def, tc.Data))
		})
	}
}
