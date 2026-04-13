package utils

// IsBinary checks if the given byte slice contains binary data by looking for null bytes.
func IsBinary(data []byte) bool {
	checkSize := min(len(data), 8192)
	nullBytes := 0

	for idx := range checkSize {
		b := data[idx]
		if b == 0 {
			nullBytes++
		}

		if nullBytes > 0 && idx > 0 && float64(nullBytes)/float64(idx) > 0.01 {
			return true
		}
	}

	return false
}
