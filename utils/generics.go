package utils

import (
	"strconv"
)

func String2Int[T ~int | ~int32 | ~int64](s string) (T, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, Wrap(err)
	}
	return T(n), nil
}
