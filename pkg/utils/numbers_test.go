package utils_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestCommaFormatInt(t *testing.T) {
	t.Parallel()

	type likeInt int

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "one",
			input:    int8(1),
			expected: "1",
		},
		{
			name:     "-100",
			input:    int(-100),
			expected: "-100",
		},
		{
			name:     "100k",
			input:    int32(100_000),
			expected: "100,000",
		},
		{
			name:     "-100m",
			input:    likeInt(-100_000_000),
			expected: "-100,000,000",
		},
		{
			name:     "1b",
			input:    int64(1_000_000_000),
			expected: "1,000,000,000",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			switch val := test.input.(type) {
			case int:
				assert.Equal(t, test.expected, utils.CommaFormatInt(val))
			case int8:
				assert.Equal(t, test.expected, utils.CommaFormatInt(val))
			case int16:
				assert.Equal(t, test.expected, utils.CommaFormatInt(val))
			case int32:
				assert.Equal(t, test.expected, utils.CommaFormatInt(val))
			case int64:
				assert.Equal(t, test.expected, utils.CommaFormatInt(val))
			case likeInt:
				assert.Equal(t, test.expected, utils.CommaFormatInt(val))
			default:
				t.Fatalf("Got non-int %T: %+v", val, val)
			}
		})
	}
}
