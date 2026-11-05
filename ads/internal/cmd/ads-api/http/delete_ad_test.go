package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newDeleteAdServer(fn usecase.DeleteAdFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("DELETE /ads/{adID}", middleware.RequireAuth(handlerDeleteAd(fn)))

	return httptest.NewServer(mux)
}

func sendDelete(t *testing.T, url, authHeader string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

func TestHandlerDeleteAd(t *testing.T) {
	t.Run("returns 204 and passes ad ID and authenticated user to the usecase", func(t *testing.T) {
		var received usecase.DeleteAdRequest
		srv := newDeleteAdServer(func(_ context.Context, req usecase.DeleteAdRequest) error {
			received = req
			return nil
		})
		defer srv.Close()

		resp := sendDelete(t, srv.URL+"/ads/42", "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Equal(t, usecase.DeleteAdRequest{ID: domain.AdID(42), RequesterID: "user-1"}, received)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ usecase.DeleteAdRequest) error {
			t.Fatal("usecase should not be called")
			return nil
		})
		defer srv.Close()

		resp := sendDelete(t, srv.URL+"/ads/42", "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 when adID is not a number", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ usecase.DeleteAdRequest) error {
			t.Fatal("usecase should not be called")
			return nil
		})
		defer srv.Close()

		resp := sendDelete(t, srv.URL+"/ads/not-a-number", "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	errorCases := []struct {
		name string
		err  error
		want int
	}{
		{name: "returns 404 when the ad does not exist", err: adsdao.ErrNotFound, want: http.StatusNotFound},
		{name: "returns 403 when the ad belongs to someone else", err: usecase.ErrForbidden, want: http.StatusForbidden},
		{name: "returns 500 on unexpected usecase error", err: errors.New("db down"), want: http.StatusInternalServerError},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newDeleteAdServer(func(_ context.Context, _ usecase.DeleteAdRequest) error {
				return tc.err
			})
			defer srv.Close()

			resp := sendDelete(t, srv.URL+"/ads/42", "user-1")
			defer resp.Body.Close()

			assert.Equal(t, tc.want, resp.StatusCode)
		})
	}
}
