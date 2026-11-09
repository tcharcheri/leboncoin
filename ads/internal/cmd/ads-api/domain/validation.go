package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Limits applied to every ad, whatever the way it is created.
const (
	MaxTitleLength    = 200
	MaxPhotoURLLength = 2048
	// MaxPriceCents is 100 million euros: far above any real ad, low enough to catch typos and overflows.
	MaxPriceCents = 100_000_000_00
)

// ErrInvalidAd is returned when an ad breaks a business rule. The wrapping error tells which one.
var ErrInvalidAd = errors.New("invalid ad")

// ValidateAd checks the fields a user provides when publishing an ad.
func ValidateAd(title string, priceCents int64, photoURL string) error {
	switch {
	case strings.TrimSpace(title) == "":
		return fmt.Errorf("%w: title is required", ErrInvalidAd)
	case utf8.RuneCountInString(title) > MaxTitleLength:
		return fmt.Errorf("%w: title is longer than %d characters", ErrInvalidAd, MaxTitleLength)
	case priceCents < 0:
		return fmt.Errorf("%w: price must not be negative", ErrInvalidAd)
	case priceCents > MaxPriceCents:
		return fmt.Errorf("%w: price is above the maximum", ErrInvalidAd)
	}

	return validatePhotoURL(photoURL)
}

func validatePhotoURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: photo_url is required", ErrInvalidAd)
	}

	if len(raw) > MaxPhotoURLLength {
		return fmt.Errorf("%w: photo_url is longer than %d characters", ErrInvalidAd, MaxPhotoURLLength)
	}

	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("%w: photo_url must be an absolute http(s) URL", ErrInvalidAd)
	}

	return nil
}
