package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

func adIDFromURL(r *http.Request) (domain.AdID, error) {
	raw := r.PathValue("adID")

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ad id %q is not a number", raw)
	}

	return domain.AdID(id), nil
}

// errBodyTooLarge is returned by decodeBody when the body exceeds the router limit.
var errBodyTooLarge = errors.New("request body too large")

// decodeBody reads exactly one JSON value with only known fields.
func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errBodyTooLarge
		}

		return fmt.Errorf("unable to decode request body: %w", err)
	}

	// Trailing data ({"a":1}{"b":2}) is a malformed request, not something to ignore silently.
	if decoder.More() {
		return errors.New("unable to decode request body: unexpected data after the JSON value")
	}

	return nil
}

// writeDecodeError answers 413 when the body is too large, 400 otherwise.
func writeDecodeError(rw http.ResponseWriter, err error) {
	if errors.Is(err, errBodyTooLarge) {
		writeError(rw, http.StatusRequestEntityTooLarge, err.Error())
		return
	}

	writeError(rw, http.StatusBadRequest, err.Error())
}

// errorf builds a client-facing validation error.
func errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// isUUID reports whether s has the canonical UUID form, e.g. 00000000-0000-0000-0000-000000000001.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}

	for i, c := range s {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F'):
			return false
		}
	}

	return true
}
