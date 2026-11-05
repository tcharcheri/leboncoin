package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestDeleteAd(t *testing.T) {
	ctx := context.Background()
	req := usecase.DeleteAdRequest{ID: 42, RequesterID: "user-1"}

	t.Run("nominal: owner deletes their ad", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(domain.Ad{ID: 42, UserID: "user-1"}, nil).Once()
		m.On("DeleteAd", ctx, domain.AdID(42), domain.UserID("user-1")).Return(nil).Once()

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		require.NoError(t, err)
	})

	t.Run("forbidden when the ad belongs to someone else", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(domain.Ad{ID: 42, UserID: "user-2"}, nil).Once()

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		require.ErrorIs(t, err, usecase.ErrForbidden)
		m.AssertNotCalled(t, "DeleteAd")
	})

	t.Run("not found when the ad does not exist", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(domain.Ad{}, adsdao.ErrNotFound).Once()

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		require.ErrorIs(t, err, adsdao.ErrNotFound)
		m.AssertNotCalled(t, "DeleteAd")
	})

	t.Run("not found when the ad disappears between read and delete", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(domain.Ad{ID: 42, UserID: "user-1"}, nil).Once()
		m.On("DeleteAd", ctx, domain.AdID(42), domain.UserID("user-1")).Return(adsdao.ErrNotFound).Once()

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		require.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("wraps DAO errors", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(domain.Ad{ID: 42, UserID: "user-1"}, nil).Once()
		m.On("DeleteAd", ctx, domain.AdID(42), domain.UserID("user-1")).Return(dbErr).Once()

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		require.ErrorIs(t, err, dbErr)
		assert.Contains(t, err.Error(), "unable to delete ad 42")
	})
}
