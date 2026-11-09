package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

// maxOffset bounds how deep a client can page: deep OFFSETs make the database skip many rows.
const maxOffset = 10_000

type listAdsItem struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	PriceCents int64     `json:"price_cents"` // Deprecated: use Price.
	Price      price     `json:"price"`
	PhotoURL   string    `json:"photo_url"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type listAdsResponse struct {
	Ads []listAdsItem `json:"ads"`
	// HasMore tells whether another page follows: request it with offset + limit.
	HasMore bool `json:"has_more"`
}

func adToListAdsItem(a domain.Ad) listAdsItem {
	return listAdsItem{
		ID:         int64(a.ID),
		Title:      a.Title,
		PriceCents: a.Price,
		Price:      price(a.Price),
		PhotoURL:   a.PhotoURL,
		UserID:     a.UserID.String(),
		CreatedAt:  a.CreatedAt.UTC(),
	}
}

// handlerListAds serves GET /ads: one page of ads, newest first.
func handlerListAds(listAds usecase.ListAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		req, err := parseListAdsQuery(r.URL.Query())
		if err != nil {
			writeError(rw, http.StatusBadRequest, err.Error())
			return
		}

		result, err := listAds(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "unable to list ads", "error", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		items := make([]listAdsItem, 0, len(result.Ads))
		for _, a := range result.Ads {
			items = append(items, adToListAdsItem(a))
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(listAdsResponse{Ads: items, HasMore: result.HasMore})
	}
}

// parseListAdsQuery validates the query string, so that a bad filter answers 400 instead of reaching the database.
func parseListAdsQuery(q url.Values) (usecase.ListAdsRequest, error) {
	req := usecase.ListAdsRequest{Title: q.Get("title")}

	if raw := q.Get("user_id"); raw != "" {
		if !isUUID(raw) {
			return req, errorf("user_id must be a UUID")
		}
		req.OwnerID = domain.UserID(raw)
	}

	var err error
	if req.MinPriceCents, err = queryInt(q, "min_price_cents", 0, domain.MaxPriceCents); err != nil {
		return req, err
	}
	if req.MaxPriceCents, err = queryInt(q, "max_price_cents", 0, domain.MaxPriceCents); err != nil {
		return req, err
	}
	if req.MaxPriceCents != 0 && req.MinPriceCents > req.MaxPriceCents {
		return req, errorf("min_price_cents must not exceed max_price_cents")
	}

	limit, err := queryInt(q, "limit", 1, usecase.MaxPageSize)
	if err != nil {
		return req, err
	}
	offset, err := queryInt(q, "offset", 0, maxOffset)
	if err != nil {
		return req, err
	}
	req.Limit, req.Offset = int(limit), int(offset)

	return req, nil
}

// queryInt reads an optional integer parameter within [lowest, highest]. It returns 0 when absent.
func queryInt(q url.Values, name string, lowest, highest int64) (int64, error) {
	raw := q.Get(name)
	if raw == "" {
		return 0, nil
	}

	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < lowest || v > highest {
		return 0, errorf("%s must be an integer between %d and %d", name, lowest, highest)
	}

	return v, nil
}
