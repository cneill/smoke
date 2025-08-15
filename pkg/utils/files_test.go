package utils_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cneill/smoke/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestWithLineNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected string
		start    int
	}{
		{
			name:     "empty",
			content:  "",
			expected: "",
		},
		{
			name:     "one_line",
			content:  "test",
			expected: "1: test\n",
		},
		{
			name:     "multi_line",
			content:  "test1\ntest2\ntest3",
			expected: "1: test1\n2: test2\n3: test3\n",
		},
		{
			name:     "many_lines",
			content:  strings.Repeat("test\n", 10),
			expected: " 1: test\n 2: test\n 3: test\n 4: test\n 5: test\n 6: test\n 7: test\n 8: test\n 9: test\n10: test\n",
		},
		{
			name:     "multi_line_with_high_start",
			content:  "test1\ntest2\ntest3",
			expected: " 999: test1\n1000: test2\n1001: test3\n",
			start:    999,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			lines := bytes.Split([]byte(test.content), []byte("\n"))

			result := utils.WithLineNumbers(lines)
			if test.start > 0 {
				result = utils.WithLineNumbers(lines, test.start)
			}

			if len(test.expected) > 0 {
				assert.Equal(t, []byte(test.expected), result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}
