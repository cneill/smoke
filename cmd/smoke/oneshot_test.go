package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: true},
		{name: "whitespace_only", input: " \n\t\n ", wantErr: true},
		{name: "plain", input: "explain main.go", want: "explain main.go"},
		{name: "padded_multiline", input: "\n  first line\nsecond line  \n\n", want: "first line\nsecond line"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := readPrompt(strings.NewReader(test.input))
			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
