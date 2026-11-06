package http

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
)

// price is a euro amount exposed as a JSON number with exactly two decimals, e.g. 120.50.
//
// It holds integer cents and never goes through float64, so the API cannot introduce
// rounding errors (0.1 + 0.2 != 0.3 in binary floating point). Cents remain the source
// of truth in the domain and the database.
type price int64

var errInvalidPrice = errors.New("price must be a non-negative number with at most two decimals")

// MarshalJSON writes the amount as a JSON number with two decimals.
func (p price) MarshalJSON() ([]byte, error) {
	cents := int64(p)
	if cents < 0 {
		return nil, fmt.Errorf("negative price %d", cents)
	}

	return fmt.Appendf(nil, "%d.%02d", cents/100, cents%100), nil
}

// UnmarshalJSON reads a JSON number such as 120, 120.5 or 120.50 into cents.
// Exponents, strings, negative amounts and more than two decimals are rejected.
func (p *price) UnmarshalJSON(data []byte) error {
	units, decimals, hasDot := bytes.Cut(data, []byte("."))
	if len(units) == 0 || len(decimals) > 2 || (hasDot && len(decimals) == 0) {
		return errInvalidPrice
	}

	if !isDigits(units) || !isDigits(decimals) || (len(units) > 1 && units[0] == '0') {
		return errInvalidPrice
	}

	euros, err := strconv.ParseInt(string(units), 10, 64)
	if err != nil || euros > math.MaxInt64/100-1 {
		return errInvalidPrice
	}

	cents := int64(0)
	for i := range 2 {
		cents *= 10
		if i < len(decimals) {
			cents += int64(decimals[i] - '0')
		}
	}

	*p = price(euros*100 + cents)

	return nil
}

func isDigits(b []byte) bool {
	for _, c := range b {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}
