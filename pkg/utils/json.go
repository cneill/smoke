package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// UnmarshalUseNumber decodes JSON from data into dst with UseNumber enabled so numeric values decode as json.Number
// rather than float64.
func UnmarshalUseNumber(data []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	return nil
}

// DecodeJSONObject decodes a JSON object into a map while preserving numbers as json.Number so re-serialized output
// stays identical. A JSON null decodes to an empty map.
func DecodeJSONObject(data []byte) (map[string]any, error) {
	var obj map[string]any
	if err := UnmarshalUseNumber(data, &obj); err != nil {
		return nil, err
	}

	if obj == nil {
		obj = map[string]any{}
	}

	return obj, nil
}
