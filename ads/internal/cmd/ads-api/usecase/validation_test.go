package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

// Invalid ads are rejected before any database call: no transaction, no lock, no quota consumed.
func TestInvalidAdsNeverReachTheDatabase(t *testing.T) {
	ctx := context.Background()

	t.Run("create", func(t *testing.T) {
		m := mockdao.New(t)

		_, err := usecase.NewCreateAdFunc(m)(ctx, usecase.CreateAdRequest{
			Title: "", PriceCents: 100, PhotoURL: "https://example.com/a.jpg", OwnerID: "user-1",
		})

		require.ErrorIs(t, err, domain.ErrInvalidAd)
		m.AssertNotCalled(t, "Begin")
	})

	t.Run("import: one invalid ad rejects the whole import", func(t *testing.T) {
		m := mockdao.New(t)

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{
			OwnerID: "user-1",
			Ads: []usecase.ImportAdItem{
				{Title: "chair", PriceCents: 1500, PhotoURL: "https://example.com/chair.jpg"},
				{Title: "table", PriceCents: -1, PhotoURL: "https://example.com/table.jpg"},
			},
		})

		require.ErrorIs(t, err, domain.ErrInvalidAd)
		assert.Contains(t, err.Error(), "ad #1")
		m.AssertNotCalled(t, "Begin")
	})
}
