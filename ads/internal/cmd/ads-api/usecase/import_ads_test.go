package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestImportAds(t *testing.T) {
	ctx := context.Background()
	items := []usecase.ImportAdItem{
		{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
		{Title: "car", PriceCents: 200000, PhotoURL: "https://example.com/car.jpg"},
	}
	daoReq := func(i int) adsdao.CreateAdRequest {
		return adsdao.CreateAdRequest{Title: items[i].Title, PriceCents: items[i].PriceCents, PhotoURL: items[i].PhotoURL, OwnerID: "user-1"}
	}

	t.Run("creates every ad in one transaction and commits", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReq(0)).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReq(1)).Return(domain.AdID(11), nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once() // deferred guard, a no-op after Commit

		got, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: items})

		require.NoError(t, err)
		assert.Equal(t, usecase.ImportAdsResult{IDs: []domain.AdID{10, 11}}, got)
	})

	t.Run("rolls everything back when one ad fails", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReq(0)).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReq(1)).Return(domain.AdID(0), errors.New("db down")).Once()
		m.On("Rollback").Return(nil).Once()

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: items})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to import ad #1")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("records the result under the idempotency key", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("Begin", ctx).Return(nil).Once()
		m.On("ClaimImport", ctx, domain.UserID("user-1"), "key-1", mock.AnythingOfType("string")).
			Return(adsdao.ImportRecord{}, true, nil).Once()
		m.On("CreateAd", ctx, daoReq(0)).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReq(1)).Return(domain.AdID(11), nil).Once()
		m.On("CompleteImport", ctx, domain.UserID("user-1"), "key-1", []domain.AdID{10, 11}).Return(nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", IdempotencyKey: "key-1", Ads: items})

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10, 11}, got.IDs)
		assert.False(t, got.Replayed)
	})

	t.Run("replays a previous import without creating anything", func(t *testing.T) {
		m := mockdao.New(t)
		var firstHash string
		m.On("Begin", ctx).Return(nil)
		m.On("ClaimImport", ctx, domain.UserID("user-1"), "key-1", mock.AnythingOfType("string")).
			Run(func(args mock.Arguments) { firstHash = args.String(3) }).
			Return(adsdao.ImportRecord{}, true, nil).Once()
		m.On("CreateAd", ctx, mock.Anything).Return(domain.AdID(10), nil).Twice()
		m.On("CompleteImport", ctx, domain.UserID("user-1"), "key-1", mock.Anything).Return(nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil)

		importAds := usecase.NewImportAdsFunc(m)
		req := usecase.ImportAdsRequest{OwnerID: "user-1", IdempotencyKey: "key-1", Ads: items}
		_, err := importAds(ctx, req)
		require.NoError(t, err)

		// Retry: the key is already claimed by an import with the same payload.
		m.On("ClaimImport", ctx, domain.UserID("user-1"), "key-1", firstHash).
			Return(adsdao.ImportRecord{RequestHash: firstHash, AdIDs: []domain.AdID{10, 11}}, false, nil).Once()

		got, err := importAds(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, usecase.ImportAdsResult{IDs: []domain.AdID{10, 11}, Replayed: true}, got)
		m.AssertNumberOfCalls(t, "CreateAd", 2) // nothing created by the retry
	})

	t.Run("rejects a key reused with a different payload", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("Begin", ctx).Return(nil).Once()
		m.On("ClaimImport", ctx, domain.UserID("user-1"), "key-1", mock.AnythingOfType("string")).
			Return(adsdao.ImportRecord{RequestHash: "another-payload", AdIDs: []domain.AdID{1}}, false, nil).Once()
		m.On("Rollback").Return(nil).Once()

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", IdempotencyKey: "key-1", Ads: items})

		require.ErrorIs(t, err, usecase.ErrIdempotencyKeyReused)
		m.AssertNotCalled(t, "CreateAd")
	})

	t.Run("rejects imports above the size limit without touching the database", func(t *testing.T) {
		m := mockdao.New(t)

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{
			OwnerID: "user-1",
			Ads:     make([]usecase.ImportAdItem, usecase.MaxImportSize+1),
		})

		require.ErrorIs(t, err, usecase.ErrTooManyAds)
		m.AssertNotCalled(t, "Begin")
	})

	t.Run("fails when the transaction cannot be committed", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReq(0)).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReq(1)).Return(domain.AdID(11), nil).Once()
		m.On("Commit").Return(errors.New("connection lost")).Once()
		m.On("Rollback").Return(nil).Once()

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: items})

		require.Error(t, err)
	})
}
