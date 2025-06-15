package base62

import (
	"math"
	"strings"
)

var alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func Encode(num int64) string {
	if num == 0 {
		return "0"
	}

	var encoded string

	for num > 0 {
		remainder := num % 62
		encoded = string(alphabet[remainder]) + encoded
		num /= 62
	}

	return encoded
}

func Decode(encoded string) int64 {
	var decoded int64

	for pos, char := range encoded {
		idx := int64(strings.Index(alphabet, string(char)))
		pwr := len(encoded) - pos - 1
		decoded += idx * int64(math.Pow(62, float64(pwr)))
	}

	return decoded
}
