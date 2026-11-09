package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// MaxImportSize bounds the number of ads in one import, and so the size and duration of its transaction.
const MaxImportSize = 1000

var (
	// ErrTooManyAds is returned when an import holds more than MaxImportSize ads.
	ErrTooManyAds = fmt.Errorf("an import holds at most %d ads", MaxImportSize)
	// ErrIdempotencyKeyReused is returned when a key is sent again with a different payload.
	ErrIdempotencyKeyReused = errors.New("idempotency key already used with a different payload")
)

type (
	// ImportAdsFunc creates every ad of an import, or none of them.
	ImportAdsFunc func(ctx context.Context, req ImportAdsRequest) (ImportAdsResult, error)

	// ImportAdsRequest describes a bulk import made by OwnerID.
	ImportAdsRequest struct {
		OwnerID domain.UserID
		// IdempotencyKey is optional. When set, retrying the same import returns the original result.
		IdempotencyKey string
		Ads            []ImportAdItem
	}

	// ImportAdItem is one ad of an import.
	ImportAdItem struct {
		Title      string `json:"title"`
		PriceCents int64  `json:"price_cents"`
		PhotoURL   string `json:"photo_url"`
	}

	// ImportAdsResult lists the created ads, in the order of the request.
	ImportAdsResult struct {
		IDs []domain.AdID
		// Replayed is true when the result comes from a previous import with the same idempotency key.
		Replayed bool
	}

	importAds struct {
		adDAO adsdao.DAO
	}
)

// NewImportAdsFunc returns an ImportAdsFunc backed by the given DAO.
func NewImportAdsFunc(adDAO adsdao.DAO) ImportAdsFunc {
	return importAds{adDAO: adDAO}.importAds
}

func (uc importAds) importAds(ctx context.Context, req ImportAdsRequest) (ImportAdsResult, error) {
	if len(req.Ads) > MaxImportSize {
		return ImportAdsResult{}, ErrTooManyAds
	}

	// Validate every ad before opening the transaction: a bad ad rejects the whole import up front.
	for i, item := range req.Ads {
		if err := domain.ValidateAd(item.Title, item.PriceCents, item.PhotoURL); err != nil {
			return ImportAdsResult{}, fmt.Errorf("ad #%d: %w", i, err)
		}
	}

	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return ImportAdsResult{}, fmt.Errorf("unable to import ads: %w", err)
	}
	// All or nothing: any early return rolls the whole import back. A no-op after Commit.
	defer func() { _ = tx.Rollback() }()

	if req.IdempotencyKey != "" {
		hash, err := requestHash(req.Ads)
		if err != nil {
			return ImportAdsResult{}, fmt.Errorf("unable to import ads: %w", err)
		}

		previous, claimed, err := tx.ClaimImport(ctx, req.OwnerID, req.IdempotencyKey, hash)
		if err != nil {
			return ImportAdsResult{}, fmt.Errorf("unable to import ads: %w", err)
		}

		if !claimed {
			if previous.RequestHash != hash {
				return ImportAdsResult{}, ErrIdempotencyKeyReused
			}

			return ImportAdsResult{IDs: previous.AdIDs, Replayed: true}, nil
		}
	}

	ids := make([]domain.AdID, 0, len(req.Ads))
	for i, item := range req.Ads {
		id, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
			Title:      item.Title,
			PriceCents: item.PriceCents,
			PhotoURL:   item.PhotoURL,
			OwnerID:    req.OwnerID,
		})
		if err != nil {
			return ImportAdsResult{}, fmt.Errorf("unable to import ad #%d: %w", i, err)
		}

		ids = append(ids, id)
	}

	if req.IdempotencyKey != "" {
		if err := tx.CompleteImport(ctx, req.OwnerID, req.IdempotencyKey, ids); err != nil {
			return ImportAdsResult{}, fmt.Errorf("unable to import ads: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return ImportAdsResult{}, fmt.Errorf("unable to import ads: %w", err)
	}

	return ImportAdsResult{IDs: ids}, nil
}

// requestHash fingerprints the payload, so a key reused with different ads is detected.
func requestHash(ads []ImportAdItem) (string, error) {
	payload, err := json.Marshal(ads)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}
