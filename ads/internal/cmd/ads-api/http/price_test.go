package http

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriceMarshalJSON(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{cents: 0, want: "0.00"},
		{cents: 5, want: "0.05"},
		{cents: 100, want: "1.00"},
		{cents: 12050, want: "120.50"},
		{cents: 349000, want: "3490.00"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got, err := json.Marshal(price(tc.cents))

			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
			assert.Equal(t, tc.want, string(got), "always two decimals")
		})
	}

	t.Run("rejects negative amounts", func(t *testing.T) {
		_, err := json.Marshal(price(-1))

		require.Error(t, err)
	})
}

func TestPriceUnmarshalJSON(t *testing.T) {
	valid := []struct {
		in   string
		want int64
	}{
		{in: "0", want: 0},
		{in: "120", want: 12000},
		{in: "120.5", want: 12050},
		{in: "120.50", want: 12050},
		{in: "0.05", want: 5},
		{in: "0.1", want: 10},
	}

	for _, tc := range valid {
		t.Run("accepts "+tc.in, func(t *testing.T) {
			var p price

			require.NoError(t, json.Unmarshal([]byte(tc.in), &p))
			assert.Equal(t, price(tc.want), p)
		})
	}

	invalid := []string{
		`120.505`,              // sub-cent precision
		`-1`,                   // negative
		`1e3`,                  // exponent
		`"120.50"`,             // string
		`120.`,                 // dangling dot
		`.5`,                   // missing units
		`012`,                  // leading zero
		`null`,                 // not a number
		`99999999999999999999`, // overflow
	}

	for _, in := range invalid {
		t.Run("rejects "+in, func(t *testing.T) {
			var p price

			require.Error(t, json.Unmarshal([]byte(in), &p))
		})
	}
}
