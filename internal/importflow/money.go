package importflow

import (
	"math/big"
	"regexp"
	"strings"
)

var decimalPattern = regexp.MustCompile(`^[+-]?\d+(?:\.\d+)?$`)

func normalizeDecimal(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) > 40 || !decimalPattern.MatchString(value) {
		return "", false
	}
	negative := strings.HasPrefix(value, "-")
	unsigned := strings.TrimLeft(value, "+-")
	parts := strings.SplitN(unsigned, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if fraction != "" {
		unsigned = integer + "." + fraction
	} else {
		unsigned = integer
	}
	if negative && unsigned != "0" {
		return "-" + unsigned, true
	}
	return unsigned, true
}

func decimalRat(value string) (*big.Rat, bool) {
	value, ok := normalizeDecimal(value)
	if !ok {
		return nil, false
	}
	result, ok := new(big.Rat).SetString(value)
	return result, ok
}
