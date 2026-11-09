package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

const (
	// PublicationQuota is the number of ads a user can publish through CreateAd per PublicationWindow.
	PublicationQuota = 5
	// PublicationWindow is the rolling window of the publication quota.
	PublicationWindow = 24 * time.Hour
)

// ErrQuotaExceeded is returned when the user already published PublicationQuota ads over the window.
// The returned error is a *QuotaExceededError, which tells when the next slot frees up.
var ErrQuotaExceeded = errors.New("publication quota exceeded")

// QuotaExceededError tells the caller when it can publish again.
type QuotaExceededError struct {
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("%s, retry in %s", ErrQuotaExceeded, e.RetryAfter.Round(time.Second))
}

// Is makes errors.Is(err, ErrQuotaExceeded) match.
func (e *QuotaExceededError) Is(target error) bool { return target == ErrQuotaExceeded }

type (
	// CreateAdFunc creates an ad owned by the calling user, within the publication quota.
	CreateAdFunc func(ctx context.Context, req CreateAdRequest) (domain.AdID, error)

	// CreateAdRequest describes the ad to create. OwnerID is the authenticated user: a caller can never create an ad on behalf of someone else.
	CreateAdRequest struct {
		Title      string
		PriceCents int64
		PhotoURL   string
		OwnerID    domain.UserID
	}

	createAd struct {
		adDAO adsdao.DAO
	}
)

// NewCreateAdFunc returns a CreateAdFunc backed by the given DAO.
func NewCreateAdFunc(adDAO adsdao.DAO) CreateAdFunc {
	return createAd{adDAO: adDAO}.createAd
}

func (uc createAd) createAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error) {
	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Without the lock, two concurrent requests could both read 4 publications and both publish.
	if err := tx.LockPublications(ctx, req.OwnerID); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	recent, err := tx.RecentPublications(ctx, req.OwnerID, PublicationWindow)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if recent.Count >= PublicationQuota {
		return 0, &QuotaExceededError{RetryAfter: recent.NextSlotIn}
	}

	adID, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
		Title:      req.Title,
		PriceCents: req.PriceCents,
		PhotoURL:   req.PhotoURL,
		OwnerID:    req.OwnerID,
	})
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if err := tx.RecordPublication(ctx, req.OwnerID, adID); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return adID, nil
}
