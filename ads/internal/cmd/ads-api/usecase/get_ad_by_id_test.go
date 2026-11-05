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
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
	usersmock "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/mock"
)

// TestGetAdByID checks that the usecase queries the DAO with the requested ID and
// enriches the ad with its owner's username.
//
// Expectations use the exact ID instead of mock.Anything: matching any argument
// would let a usecase that ignores its input pass the tests.
func TestGetAdByID(t *testing.T) {
	ctx := context.Background()
	ad := domain.Ad{ID: 42, Title: "bike", Price: 5000, UserID: "user-1"}

	t.Run("nominal: returns the ad with its owner's username", func(t *testing.T) {
		ads := mockdao.New(t)
		users := usersmock.New(t)
		ads.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil).Once()
		users.On("GetUserByID", ctx, model.UserID("user-1")).
			Return(usersclient.User{UserID: "user-1", Nickname: "alice"}, nil).Once()

		got, err := usecase.NewGetAdByIDFunc(ads, users)(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, domain.AdDetails{Ad: ad, Username: "alice"}, got)
	})

	t.Run("degrades to an empty username when the owner does not exist", func(t *testing.T) {
		ads := mockdao.New(t)
		users := usersmock.New(t)
		ads.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil).Once()
		users.On("GetUserByID", ctx, model.UserID("user-1")).Return(usersclient.User{}, usersapi.ErrNotFound).Once()

		got, err := usecase.NewGetAdByIDFunc(ads, users)(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, domain.AdDetails{Ad: ad}, got)
	})

	t.Run("degrades to an empty username when the user service fails", func(t *testing.T) {
		ads := mockdao.New(t)
		users := usersmock.New(t)
		ads.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil).Once()
		users.On("GetUserByID", ctx, model.UserID("user-1")).Return(usersclient.User{}, errors.New("timeout")).Once()

		got, err := usecase.NewGetAdByIDFunc(ads, users)(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, domain.AdDetails{Ad: ad}, got)
	})

	t.Run("error no ad found: the user service is not called", func(t *testing.T) {
		ads := mockdao.New(t)
		users := usersmock.New(t)
		ads.On("GetAdByID", ctx, domain.AdID(99)).Return(domain.Ad{}, adsdao.ErrNotFound).Once()

		_, err := usecase.NewGetAdByIDFunc(ads, users)(ctx, 99)

		require.ErrorIs(t, err, adsdao.ErrNotFound)
		users.AssertNotCalled(t, "GetUserByID")
	})

	t.Run("error from dao is wrapped with the ad ID", func(t *testing.T) {
		ads := mockdao.New(t)
		users := usersmock.New(t)
		dbErr := errors.New("connection reset")
		ads.On("GetAdByID", ctx, domain.AdID(1)).Return(domain.Ad{}, dbErr).Once()

		_, err := usecase.NewGetAdByIDFunc(ads, users)(ctx, 1)

		require.ErrorIs(t, err, dbErr)
		assert.Contains(t, err.Error(), "unable to get ad 1")
	})
}
