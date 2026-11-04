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

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		m := mockdao.New(t)
		want := domain.Ad{ID: 42, Title: "bike", Price: 5000, UserID: "user-1"}
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(want, nil).Once()

		got, err := usecase.NewGetAdByIDFunc(m)(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("error no ad found", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("GetAdByID", ctx, domain.AdID(99)).Return(domain.Ad{}, adsdao.ErrNotFound).Once()

		_, err := usecase.NewGetAdByIDFunc(m)(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")
		m.On("GetAdByID", ctx, domain.AdID(1)).Return(domain.Ad{}, dbErr).Once()

		_, err := usecase.NewGetAdByIDFunc(m)(ctx, 1)

		require.ErrorIs(t, err, dbErr)
		assert.Contains(t, err.Error(), "unable to get ad 1")
	})
}
