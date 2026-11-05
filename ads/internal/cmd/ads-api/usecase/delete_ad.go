package usecase

import (
	"context"
	"errors"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// ErrForbidden is returned when a user acts on an ad owned by someone else.
var ErrForbidden = errors.New("ad belongs to another user")

type (
	// DeleteAdFunc deletes an ad owned by the calling user.
	// It fails with dao.ErrNotFound when the ad does not exist, and with ErrForbidden when it belongs to someone else.
	DeleteAdFunc func(ctx context.Context, req DeleteAdRequest) error

	// DeleteAdRequest describes the ad to delete. RequesterID is the authenticated user.
	DeleteAdRequest struct {
		ID          domain.AdID
		RequesterID domain.UserID
	}

	deleteAd struct {
		adDAO adsdao.DAO
	}
)

// NewDeleteAdFunc returns a DeleteAdFunc backed by the given DAO.
func NewDeleteAdFunc(adDAO adsdao.DAO) DeleteAdFunc {
	return deleteAd{adDAO: adDAO}.deleteAd
}

func (uc deleteAd) deleteAd(ctx context.Context, req DeleteAdRequest) error {
	// Reading first lets us tell "does not exist" (404) from "not yours" (403).
	ad, err := uc.adDAO.GetAdByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", req.ID, err)
	}

	if ad.UserID != req.RequesterID {
		return fmt.Errorf("unable to delete ad %s: %w", req.ID, ErrForbidden)
	}

	// The DAO re-checks the owner in the DELETE statement, so a concurrent change cannot slip through.
	if err := uc.adDAO.DeleteAd(ctx, req.ID, req.RequesterID); err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", req.ID, err)
	}

	return nil
}
