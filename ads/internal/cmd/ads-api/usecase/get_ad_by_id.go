package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	GetAdByIDFunc func(ctx context.Context, id domain.AdID) (domain.Ad, error)

	getAdByID struct {
		adDAO adsdao.DAO
	}
)

func NewGetAdByIDFunc(adDAO adsdao.DAO) GetAdByIDFunc {
	return getAdByID{adDAO: adDAO}.getAdByID
}

func (uc getAdByID) getAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error) {
	found, err := uc.adDAO.GetAdByID(ctx, id)
	if err != nil {
		return domain.Ad{}, fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	return found, nil
}
