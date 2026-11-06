package http

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func postRawJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "user-1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

func TestHandlerCreateAdPrice(t *testing.T) {
	t.Run("accepts price in euros and converts it to cents", func(t *testing.T) {
		var received usecase.CreateAdRequest
		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			received = req
			return 1, nil
		})
		defer srv.Close()

		resp := postRawJSON(t, srv.URL+"/ads", `{"title":"bike","price":120.5,"photo_url":"https://example.com/bike.jpg"}`)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, int64(12050), received.PriceCents)
	})

	invalid := map[string]string{
		"both price and price_cents": `{"title":"bike","price":120.5,"price_cents":12050}`,
		"sub-cent price":             `{"title":"bike","price":120.505}`,
		"negative price":             `{"title":"bike","price":-1}`,
	}

	for name, body := range invalid {
		t.Run("returns 400 on "+name, func(t *testing.T) {
			srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
				t.Fatal("usecase should not be called")
				return 0, nil
			})
			defer srv.Close()

			resp := postRawJSON(t, srv.URL+"/ads", body)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}
