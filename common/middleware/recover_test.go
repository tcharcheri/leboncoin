package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func TestRecover(t *testing.T) {
	t.Run("answers 500 when the handler panics", func(t *testing.T) {
		handler := middleware.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ads", nil))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("lets normal responses through", func(t *testing.T) {
		handler := middleware.Recover(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusTeapot)
		}))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ads", nil))

		assert.Equal(t, http.StatusTeapot, rec.Code)
	})
}
