package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestCreateAd(t *testing.T) {
	ctx := context.Background()

	ucReq := usecase.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}
	daoReq := adsdao.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	// withinQuota sets up a transaction where the user already published `published` ads.
	withinQuota := func(m *mock.Mock, published int) {
		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockPublications", ctx, domain.UserID("user-1")).Return(nil).Once()
		m.On("RecentPublications", ctx, domain.UserID("user-1"), 24*time.Hour).
			Return(adsdao.PublicationStats{Count: published, NextSlotIn: 3 * time.Hour}, nil).Once()
		m.On("Rollback").Return(nil).Once()
	}

	t.Run("nominal: creates the ad and records the publication in one transaction", func(t *testing.T) {
		m := mock.New(t)
		withinQuota(m, 4)
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once()
		m.On("RecordPublication", ctx, domain.UserID("user-1"), domain.AdID(7)).Return(nil).Once()
		m.On("Commit").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("rejects the 6th publication of the window and tells when to retry", func(t *testing.T) {
		m := mock.New(t)
		withinQuota(m, 5)

		_, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, usecase.ErrQuotaExceeded)

		var quotaErr *usecase.QuotaExceededError
		require.ErrorAs(t, err, &quotaErr)
		assert.Equal(t, 3*time.Hour, quotaErr.RetryAfter)
		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from dao rolls back", func(t *testing.T) {
		m := mock.New(t)
		withinQuota(m, 0)
		dbErr := errors.New("connection reset")
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(0), dbErr).Once()

		_, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error when the publication cannot be recorded rolls back the ad", func(t *testing.T) {
		m := mock.New(t)
		withinQuota(m, 0)
		dbErr := errors.New("connection reset")
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once()
		m.On("RecordPublication", ctx, domain.UserID("user-1"), domain.AdID(7)).Return(dbErr).Once()

		_, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error when the lock cannot be taken", func(t *testing.T) {
		m := mock.New(t)
		dbErr := errors.New("connection reset")
		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockPublications", ctx, domain.UserID("user-1")).Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		_, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		m.AssertNotCalled(t, "RecentPublications")
	})
}
