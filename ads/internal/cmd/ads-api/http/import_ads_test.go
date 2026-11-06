package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newImportAdsServer(fn usecase.ImportAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads/import", middleware.RequireAuth(handlerImportAds(fn)))
	return httptest.NewServer(mux)
}

func postImport(t *testing.T, url string, body any, idempotencyKey string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "user-1")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func mustNotBeCalled(t *testing.T) usecase.ImportAdsFunc {
	return func(context.Context, usecase.ImportAdsRequest) (usecase.ImportAdsResult, error) {
		t.Fatal("usecase should not be called")
		return usecase.ImportAdsResult{}, nil
	}
}

func TestHandlerImportAds(t *testing.T) {
	twoAds := importAdsBody{Ads: []createAdBody{
		{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
		{Title: "scooter", PriceCents: 8000, PhotoURL: "https://example.com/scooter.jpg"},
	}}

	t.Run("returns 201 with the IDs and passes owner, key and ads to the usecase", func(t *testing.T) {
		var received usecase.ImportAdsRequest
		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) (usecase.ImportAdsResult, error) {
			received = req
			return usecase.ImportAdsResult{IDs: []domain.AdID{10, 11}}, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", twoAds, "key-1")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("Idempotent-Replayed"))

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{10, 11}, got.IDs)

		assert.Equal(t, usecase.ImportAdsRequest{
			OwnerID:        "user-1",
			IdempotencyKey: "key-1",
			Ads: []usecase.ImportAdItem{
				{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
				{Title: "scooter", PriceCents: 8000, PhotoURL: "https://example.com/scooter.jpg"},
			},
		}, received)
	})

	t.Run("flags a replayed import", func(t *testing.T) {
		srv := newImportAdsServer(func(context.Context, usecase.ImportAdsRequest) (usecase.ImportAdsResult, error) {
			return usecase.ImportAdsResult{IDs: []domain.AdID{10, 11}, Replayed: true}, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", twoAds, "key-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "true", resp.Header.Get("Idempotent-Replayed"))
	})

	t.Run("returns 201 with empty IDs when body has no ads", func(t *testing.T) {
		srv := newImportAdsServer(func(context.Context, usecase.ImportAdsRequest) (usecase.ImportAdsResult, error) {
			return usecase.ImportAdsResult{}, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", importAdsBody{Ads: []createAdBody{}}, "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Empty(t, got.IDs)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		srv := newImportAdsServer(mustNotBeCalled(t))
		defer srv.Close()

		resp, err := http.Post(srv.URL+"/ads/import", "application/json", strings.NewReader(`{"ads":[]}`))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 on invalid body", func(t *testing.T) {
		srv := newImportAdsServer(mustNotBeCalled(t))
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", "not an object", "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 when one price is invalid, before importing anything", func(t *testing.T) {
		srv := newImportAdsServer(mustNotBeCalled(t))
		defer srv.Close()

		resp := postRawJSON(t, srv.URL+"/ads/import", `{"ads":[{"title":"bike","price_cents":5000},{"title":"car","price":1,"price_cents":100}]}`)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 when the idempotency key is too long", func(t *testing.T) {
		srv := newImportAdsServer(mustNotBeCalled(t))
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", twoAds, strings.Repeat("k", 256))
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	errorCases := []struct {
		name string
		err  error
		want int
	}{
		{name: "returns 413 when the import is too large", err: usecase.ErrTooManyAds, want: http.StatusRequestEntityTooLarge},
		{name: "returns 422 when the key is reused with another payload", err: usecase.ErrIdempotencyKeyReused, want: http.StatusUnprocessableEntity},
		{name: "returns 500 on unexpected error", err: errors.New("db down"), want: http.StatusInternalServerError},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newImportAdsServer(func(context.Context, usecase.ImportAdsRequest) (usecase.ImportAdsResult, error) {
				return usecase.ImportAdsResult{}, tc.err
			})
			defer srv.Close()

			resp := postImport(t, srv.URL+"/ads/import", twoAds, "key-1")
			defer resp.Body.Close()

			assert.Equal(t, tc.want, resp.StatusCode)
		})
	}
}
