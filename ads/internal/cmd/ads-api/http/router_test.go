package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func allUsecases() Usecases {
	return Usecases{
		ListAds: func(context.Context, usecase.ListAdsRequest) (usecase.ListAdsResult, error) {
			return usecase.ListAdsResult{}, nil
		},
		GetAd: func(context.Context, domain.AdID) (domain.AdDetails, error) { return domain.AdDetails{}, nil },
		CreateAd: func(context.Context, usecase.CreateAdRequest) (domain.AdID, error) {
			return 1, nil
		},
		DeleteAd: func(context.Context, usecase.DeleteAdRequest) error { return nil },
		ImportAds: func(context.Context, usecase.ImportAdsRequest) (usecase.ImportAdsResult, error) {
			return usecase.ImportAdsResult{}, nil
		},
	}
}

func TestNewRouter(t *testing.T) {
	t.Run("serves every route when all usecases are wired", func(t *testing.T) {
		router, err := NewRouter(allUsecases())
		require.NoError(t, err)

		routes := []struct{ method, path string }{
			{http.MethodGet, "/health"},
			{http.MethodGet, "/ads"},
			{http.MethodGet, "/ads/1"},
			{http.MethodPost, "/ads"},
			{http.MethodDelete, "/ads/1"},
			{http.MethodPost, "/ads/import"},
		}
		for _, route := range routes {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
			req.Header.Set("Authorization", "user-1")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.NotEqual(t, http.StatusNotFound, rec.Code, "%s %s", route.method, route.path)
			assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code, "%s %s", route.method, route.path)
		}
	})

	t.Run("fails at startup when a usecase is not wired", func(t *testing.T) {
		usecases := allUsecases()
		usecases.DeleteAd = nil

		_, err := NewRouter(usecases)

		require.EqualError(t, err, "usecase DeleteAd is not wired")
	})

	t.Run("rejects bodies above the size limit with 413", func(t *testing.T) {
		router, err := NewRouter(allUsecases())
		require.NoError(t, err)

		huge := `{"ads":[{"title":"` + strings.Repeat("a", maxBodyBytes) + `"}]}`
		req := httptest.NewRequest(http.MethodPost, "/ads/import", strings.NewReader(huge))
		req.Header.Set("Authorization", "user-1")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})
}
