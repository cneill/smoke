package utils_test

import (
	"encoding/json"
	"testing"

	"github.com/cneill/smoke/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJSONObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    map[string]any
		wantErr bool
	}{
		{
			name:  "object with preserved number",
			input: `{"path":"README.md","n":1.50}`,
			want:  map[string]any{"path": "README.md", "n": json.Number("1.50")},
		},
		{
			name:  "null becomes empty map",
			input: `null`,
			want:  map[string]any{},
		},
		{
			name:    "invalid json",
			input:   `{"path":`,
			wantErr: true,
		},
		{
			name:    "non-object json",
			input:   `[1,2,3]`,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := utils.DecodeJSONObject([]byte(test.input))
			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
