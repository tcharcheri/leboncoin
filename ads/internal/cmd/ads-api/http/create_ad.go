package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

type createAdBody struct {
	Title string `json:"title"`
	// PriceCents is deprecated: clients should send Price. Both are accepted during the migration.
	PriceCents int64  `json:"price_cents"`
	Price      *price `json:"price,omitempty"`
	PhotoURL   string `json:"photo_url"`
}

var errAmbiguousPrice = errors.New("send either price or price_cents, not both")

// priceInCents returns the price sent by the client, whichever field it used.
func (b createAdBody) priceInCents() (int64, error) {
	if b.Price == nil {
		return b.PriceCents, nil
	}

	if b.PriceCents != 0 {
		return 0, errAmbiguousPrice
	}

	return int64(*b.Price), nil
}

type createAdResponse struct {
	ID int64 `json:"id"`
}

// handlerCreateAd serves POST /ads.
func handlerCreateAd(createAd usecase.CreateAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body createAdBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		priceCents, err := body.priceInCents()
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		adID, err := createAd(r.Context(), usecase.CreateAdRequest{
			Title:      body.Title,
			PriceCents: priceCents,
			PhotoURL:   body.PhotoURL,
			OwnerID:    middleware.UserIDFromHeader(r),
		})
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(createAdResponse{ID: int64(adID)})
	}
}
