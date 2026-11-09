package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestListAds(t *testing.T) {
	ctx := context.Background()
	ads := func(n int) []domain.Ad {
		out := make([]domain.Ad, n)
		for i := range out {
			out[i] = domain.Ad{ID: domain.AdID(i + 1)}
		}
		return out
	}

	t.Run("forwards the filters and asks one extra row for the default page size", func(t *testing.T) {
		m := mock.New(t)
		m.On("SearchAds", ctx, adsdao.SearchAdsRequest{Title: "bike", OwnerID: "user-1", MinPriceCents: 100, MaxPriceCents: 900, Limit: 21}).
			Return(ads(2), nil).Once()

		got, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Title: "bike", OwnerID: "user-1", MinPriceCents: 100, MaxPriceCents: 900})

		require.NoError(t, err)
		assert.Equal(t, usecase.ListAdsResult{Ads: ads(2)}, got)
	})

	t.Run("reports another page when the DAO returns the extra row", func(t *testing.T) {
		m := mock.New(t)
		m.On("SearchAds", ctx, adsdao.SearchAdsRequest{Limit: 3, Offset: 10}).Return(ads(3), nil).Once()

		got, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Limit: 2, Offset: 10})

		require.NoError(t, err)
		assert.Equal(t, usecase.ListAdsResult{Ads: ads(2), HasMore: true}, got)
	})

	t.Run("caps the page size", func(t *testing.T) {
		m := mock.New(t)
		m.On("SearchAds", ctx, adsdao.SearchAdsRequest{Limit: usecase.MaxPageSize + 1}).Return(nil, nil).Once()

		got, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Limit: 10_000})

		require.NoError(t, err)
		assert.Empty(t, got.Ads)
		assert.False(t, got.HasMore)
	})

	t.Run("wraps DAO errors", func(t *testing.T) {
		m := mock.New(t)
		dbErr := errors.New("connection reset")
		m.On("SearchAds", ctx, adsdao.SearchAdsRequest{Limit: 21}).Return(nil, dbErr).Once()

		_, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{})

		require.ErrorIs(t, err, dbErr)
	})
}
