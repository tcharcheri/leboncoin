package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

func TestValidateAd(t *testing.T) {
	const photo = "https://example.com/bike.jpg"

	t.Run("accepts a valid ad", func(t *testing.T) {
		require.NoError(t, domain.ValidateAd("Mountain bike", 12050, photo))
		require.NoError(t, domain.ValidateAd("Free chair", 0, "http://example.com/chair.jpg"))
		require.NoError(t, domain.ValidateAd(strings.Repeat("é", domain.MaxTitleLength), domain.MaxPriceCents, photo))
	})

	invalid := []struct {
		name     string
		title    string
		price    int64
		photoURL string
		message  string
	}{
		{name: "empty title", title: "", price: 100, photoURL: photo, message: "title is required"},
		{name: "blank title", title: "   ", price: 100, photoURL: photo, message: "title is required"},
		{name: "title too long", title: strings.Repeat("a", domain.MaxTitleLength+1), price: 100, photoURL: photo, message: "title is longer"},
		{name: "negative price", title: "bike", price: -1, photoURL: photo, message: "must not be negative"},
		{name: "price too high", title: "bike", price: domain.MaxPriceCents + 1, photoURL: photo, message: "above the maximum"},
		{name: "missing photo", title: "bike", price: 100, photoURL: "", message: "photo_url is required"},
		{name: "relative photo", title: "bike", price: 100, photoURL: "/photos/bike.jpg", message: "absolute http(s) URL"},
		{name: "unsafe scheme", title: "bike", price: 100, photoURL: "javascript:alert(1)", message: "absolute http(s) URL"},
		{name: "photo URL too long", title: "bike", price: 100, photoURL: "https://example.com/" + strings.Repeat("a", domain.MaxPhotoURLLength), message: "photo_url is longer"},
	}

	for _, tc := range invalid {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			err := domain.ValidateAd(tc.title, tc.price, tc.photoURL)

			require.ErrorIs(t, err, domain.ErrInvalidAd)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}
