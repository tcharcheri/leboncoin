package http

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestHandlerCreateAdQuota(t *testing.T) {
	t.Run("returns 429 with Retry-After when the quota is exceeded", func(t *testing.T) {
		srv := newCreateAdServer(func(context.Context, usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, fmt.Errorf("wrapped: %w", &usecase.QuotaExceededError{RetryAfter: 90*time.Minute + 500*time.Millisecond})
		})
		defer srv.Close()

		resp := postRawJSON(t, srv.URL+"/ads", `{"title":"bike","price":10}`)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		assert.Equal(t, "5401", resp.Header.Get("Retry-After"), "seconds, rounded up")
	})
}

func TestRetryAfterSeconds(t *testing.T) {
	assert.Equal(t, "1", retryAfterSeconds(0), "never tell a client to retry immediately")
	assert.Equal(t, "1", retryAfterSeconds(200*time.Millisecond))
	assert.Equal(t, "3600", retryAfterSeconds(time.Hour))
}
