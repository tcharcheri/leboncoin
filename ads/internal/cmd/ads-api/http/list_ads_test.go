package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func newListAdsServer(fn usecase.ListAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /ads", handlerListAds(fn))
	return httptest.NewServer(mux)
}

func listMustNotBeCalled(t *testing.T) usecase.ListAdsFunc {
	return func(context.Context, usecase.ListAdsRequest) (usecase.ListAdsResult, error) {
		t.Fatal("usecase should not be called")
		return usecase.ListAdsResult{}, nil
	}
}

func TestHandlerListAds(t *testing.T) {
	createdAt := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)

	t.Run("returns 200 with one page of ads", func(t *testing.T) {
		ads := []domain.Ad{
			{ID: 1, Title: "bike", Price: 5000, PhotoURL: "https://example.com/bike.jpg", UserID: "user-1", CreatedAt: createdAt},
			{ID: 2, Title: "scooter", Price: 12000, PhotoURL: "https://example.com/scooter.jpg", UserID: "user-2", CreatedAt: createdAt},
		}
		srv := newListAdsServer(func(context.Context, usecase.ListAdsRequest) (usecase.ListAdsResult, error) {
			return usecase.ListAdsResult{Ads: ads, HasMore: true}, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var got listAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		require.Len(t, got.Ads, 2)
		assert.True(t, got.HasMore)
		assert.Equal(t, int64(1), got.Ads[0].ID)
		assert.Equal(t, "bike", got.Ads[0].Title)
		assert.Equal(t, int64(5000), got.Ads[0].PriceCents)
		assert.Equal(t, price(5000), got.Ads[0].Price)
		assert.Equal(t, int64(2), got.Ads[1].ID)
	})

	t.Run("returns 200 with an empty ads array when no results", func(t *testing.T) {
		srv := newListAdsServer(func(context.Context, usecase.ListAdsRequest) (usecase.ListAdsResult, error) {
			return usecase.ListAdsResult{}, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var got map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []any{}, got["ads"], "an empty array, not null")
		assert.Equal(t, false, got["has_more"])
	})

	t.Run("forwards filters and pagination to the usecase", func(t *testing.T) {
		var received usecase.ListAdsRequest
		srv := newListAdsServer(func(_ context.Context, req usecase.ListAdsRequest) (usecase.ListAdsResult, error) {
			received = req
			return usecase.ListAdsResult{}, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads?title=bike&user_id=00000000-0000-0000-0000-000000000003&min_price_cents=1000&max_price_cents=50000&limit=10&offset=30")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, usecase.ListAdsRequest{
			Title:         "bike",
			OwnerID:       "00000000-0000-0000-0000-000000000003",
			MinPriceCents: 1000,
			MaxPriceCents: 50000,
			Limit:         10,
			Offset:        30,
		}, received)
	})

	badQueries := map[string]string{
		"min_price_cents is not a number":   "min_price_cents=not-a-number",
		"max_price_cents is not a number":   "max_price_cents=abc",
		"negative price":                    "min_price_cents=-1",
		"min price above max price":         "min_price_cents=5000&max_price_cents=1000",
		"user_id is not a UUID":             "user_id=user-3",
		"limit is zero":                     "limit=0",
		"limit is above the maximum":        "limit=101",
		"offset is negative":                "offset=-1",
		"offset is beyond the deepest page": "offset=10001",
	}

	for name, query := range badQueries {
		t.Run("returns 400 when "+name, func(t *testing.T) {
			srv := newListAdsServer(listMustNotBeCalled(t))
			defer srv.Close()

			resp, err := http.Get(srv.URL + "/ads?" + query)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var body errorResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.NotEmpty(t, body.Error)
		})
	}

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newListAdsServer(func(context.Context, usecase.ListAdsRequest) (usecase.ListAdsResult, error) {
			return usecase.ListAdsResult{}, errors.New("db down")
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
