package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

const (
	// headerIdempotencyKey lets a client retry an import safely: the same key returns the original result.
	headerIdempotencyKey = "Idempotency-Key"
	// headerIdempotentReplayed tells the client that the response comes from a previous import.
	headerIdempotentReplayed = "Idempotent-Replayed"
	maxIdempotencyKeyLength  = 255
)

type importAdsBody struct {
	Ads []createAdBody `json:"ads"`
}

type importAdsResponse struct {
	IDs []int64 `json:"ids"`
}

// handlerImportAds serves POST /ads/import. The import is atomic, and idempotent when an Idempotency-Key header is sent.
func handlerImportAds(importAds usecase.ImportAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(headerIdempotencyKey)
		if len(key) > maxIdempotencyKeyLength {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		var body importAdsBody
		if err := decodeBody(r, &body); err != nil {
			writeDecodeError(rw, err)
			return
		}

		items := make([]usecase.ImportAdItem, len(body.Ads))
		for i, ad := range body.Ads {
			priceCents, err := ad.priceInCents()
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			items[i] = usecase.ImportAdItem{Title: ad.Title, PriceCents: priceCents, PhotoURL: ad.PhotoURL}
		}

		result, err := importAds(r.Context(), usecase.ImportAdsRequest{
			OwnerID:        middleware.UserIDFromRequest(r),
			IdempotencyKey: key,
			Ads:            items,
		})

		switch {
		case err == nil:
		case errors.Is(err, domain.ErrInvalidAd):
			writeError(rw, http.StatusBadRequest, err.Error())
			return
		case errors.Is(err, usecase.ErrTooManyAds):
			rw.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		case errors.Is(err, usecase.ErrIdempotencyKeyReused):
			rw.WriteHeader(http.StatusUnprocessableEntity)
			return
		default:
			slog.ErrorContext(r.Context(), "unable to import ads", "ads", len(items), "error", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		ids := make([]int64, len(result.IDs))
		for i, id := range result.IDs {
			ids[i] = int64(id)
		}

		if result.Replayed {
			rw.Header().Set(headerIdempotentReplayed, "true")
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(importAdsResponse{IDs: ids})
	}
}
