package utils

import (
	"strconv"
	"strings"
)

type Int interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

func CommaFormatInt[T Int](input T) string {
	var sb strings.Builder

	bytes := []byte(strconv.FormatInt(int64(input), 10))
	if bytes[0] == '-' {
		bytes = bytes[1:]

		sb.WriteRune('-')
	}

	for idx, chr := range bytes {
		if idx > 0 && (len(bytes)-idx)%3 == 0 {
			sb.WriteRune(',')
		}

		sb.WriteRune(rune(chr))
	}

	return sb.String()
}
