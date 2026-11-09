package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// Page sizes of ListAds. A bounded page keeps every request cheap, whatever the size of the catalog.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type (
	// ListAdsFunc returns one page of the ads matching the filters, newest first.
	ListAdsFunc func(ctx context.Context, req ListAdsRequest) (ListAdsResult, error)

	// ListAdsRequest describes which ads to return.
	ListAdsRequest struct {
		OwnerID       domain.UserID
		MinPriceCents int64
		MaxPriceCents int64
		Title         string
		// Limit is the page size: DefaultPageSize when 0, at most MaxPageSize.
		Limit  int
		Offset int
	}

	// ListAdsResult is one page of ads.
	ListAdsResult struct {
		Ads []domain.Ad
		// HasMore tells whether another page follows.
		HasMore bool
	}

	listAds struct {
		adDAO adsdao.DAO
	}
)

// NewListAdsFunc returns a new ListAdsFunc.
func NewListAdsFunc(adDAO adsdao.DAO) ListAdsFunc {
	return listAds{adDAO: adDAO}.listAds
}

func (uc listAds) listAds(ctx context.Context, req ListAdsRequest) (ListAdsResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limit = min(limit, MaxPageSize)

	// One extra row tells whether another page exists, without a COUNT(*) over the whole table.
	ads, err := uc.adDAO.SearchAds(ctx, adsdao.SearchAdsRequest{
		OwnerID:       req.OwnerID,
		MinPriceCents: req.MinPriceCents,
		MaxPriceCents: req.MaxPriceCents,
		Title:         req.Title,
		Limit:         limit + 1,
		Offset:        max(req.Offset, 0),
	})
	if err != nil {
		return ListAdsResult{}, fmt.Errorf("unable to search ads: %w", err)
	}

	if len(ads) > limit {
		return ListAdsResult{Ads: ads[:limit], HasMore: true}, nil
	}

	return ListAdsResult{Ads: ads}, nil
}
