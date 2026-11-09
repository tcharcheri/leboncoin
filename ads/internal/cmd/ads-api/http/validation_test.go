package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestInvalidAdAnswers400WithMessage(t *testing.T) {
	invalid := fmt.Errorf("%w: title is required", domain.ErrInvalidAd)

	t.Run("create", func(t *testing.T) {
		srv := newCreateAdServer(func(context.Context, usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, invalid
		})
		defer srv.Close()

		resp := postRawJSON(t, srv.URL+"/ads", `{"title":"","price":1,"photo_url":"https://example.com/a.jpg"}`)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		var body errorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "invalid ad: title is required", body.Error)
	})

	t.Run("import", func(t *testing.T) {
		srv := newImportAdsServer(func(context.Context, usecase.ImportAdsRequest) (usecase.ImportAdsResult, error) {
			return usecase.ImportAdsResult{}, fmt.Errorf("ad #1: %w", invalid)
		})
		defer srv.Close()

		resp := postRawJSON(t, srv.URL+"/ads/import", `{"ads":[]}`)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		var body errorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "ad #1: invalid ad: title is required", body.Error)
	})
}
