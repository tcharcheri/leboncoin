package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

type (
	// GetAdByIDFunc returns a single ad with its owner's username.
	// It fails with dao.ErrNotFound when no ad matches the identifier.
	GetAdByIDFunc func(ctx context.Context, id domain.AdID) (domain.AdDetails, error)

	getAdByID struct {
		adDAO adsdao.DAO
		users usersclient.UserClient
	}
)

// NewGetAdByIDFunc returns a GetAdByIDFunc backed by the given DAO and user service client.
func NewGetAdByIDFunc(adDAO adsdao.DAO, users usersclient.UserClient) GetAdByIDFunc {
	return getAdByID{adDAO: adDAO, users: users}.getAdByID
}

func (uc getAdByID) getAdByID(ctx context.Context, id domain.AdID) (domain.AdDetails, error) {
	ad, err := uc.adDAO.GetAdByID(ctx, id)
	if err != nil {
		return domain.AdDetails{}, fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	details := domain.AdDetails{Ad: ad}

	// The username is a nice-to-have on a user-facing, heavily read page: an unavailable
	// user service must degrade the response, not fail it.
	user, err := uc.users.GetUserByID(ctx, ad.UserID)
	switch {
	case err == nil:
		details.Username = user.Nickname
	case errors.Is(err, usersapi.ErrNotFound):
		// The owner no longer exists: there is no username to show.
	default:
		slog.WarnContext(ctx, "unable to get ad owner, serving the ad without username",
			"ad_id", id, "user_id", ad.UserID, "error", err)
	}

	return details, nil
}
