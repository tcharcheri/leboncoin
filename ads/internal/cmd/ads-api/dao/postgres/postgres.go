package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

const adColumns = `id, title, price_cents, photo_url, user_id::text, created_at`

// DAO is a PostgreSQL-backed ad store.
type DAO struct {
	db *sql.DB
	// tx is set when the DAO is scoped to a transaction opened by WithTx.
	tx *sql.Tx
}

// queryRunner is the subset of *sql.DB and *sql.Tx the queries need.
type queryRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// New opens a connection pool to the given DSN and checks that the database is reachable. The caller is responsible for calling Close.
func New(ctx context.Context, dsn string) (*DAO, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("unable to reach database: %w", err)
	}

	return &DAO{db: db}, nil
}

// Close releases the connection pool.
func (d *DAO) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("unable to close database: %w", err)
	}

	return nil
}

// runner returns the transaction the DAO is scoped to, or the connection pool.
func (d *DAO) runner() queryRunner {
	if d.tx != nil {
		return d.tx
	}

	return d.db
}

// Begin opens a transaction. Every call made on the returned Tx is part of it, and the caller must end it with Commit or Rollback.
func (d *DAO) Begin(ctx context.Context) (adsdao.Tx, error) {
	if d.tx != nil {
		return nil, errors.New("unable to open a nested transaction")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}

	return &DAO{db: d.db, tx: tx}, nil
}

// Commit applies every change made through the transaction.
func (d *DAO) Commit() error {
	if d.tx == nil {
		return errors.New("unable to commit outside a transaction")
	}

	if err := d.tx.Commit(); err != nil {
		return fmt.Errorf("unable to commit transaction: %w", err)
	}

	return nil
}

// Rollback discards every change made through the transaction. Calling it on an already committed transaction does nothing, so `defer func() { _ = tx.Rollback() }()` is a safe guard.
func (d *DAO) Rollback() error {
	if d.tx == nil {
		return errors.New("unable to roll back outside a transaction")
	}

	if err := d.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("unable to roll back transaction: %w", err)
	}

	return nil
}

// SearchAds returns all ads matching the filters.
func (d *DAO) SearchAds(ctx context.Context, req adsdao.SearchAdsRequest) ([]domain.Ad, error) {
	const query = `
		SELECT ` + adColumns + `
		FROM ads
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = 0 OR price_cents >= $2)
		  AND ($3 = 0 OR price_cents <= $3)
		  AND ($4 = '' OR title ILIKE '%' || $4 || '%')`

	rows, err := d.runner().QueryContext(ctx, query,
		req.OwnerID,
		req.MinPriceCents,
		req.MaxPriceCents,
		req.Title,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to search ads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ads []domain.Ad
	for rows.Next() {
		var ad domain.Ad
		if err := rows.Scan(&ad.ID, &ad.Title, &ad.Price, &ad.PhotoURL, &ad.UserID, &ad.CreatedAt); err != nil {
			return nil, fmt.Errorf("unable to scan ad: %w", err)
		}

		ads = append(ads, ad)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unable to read ads: %w", err)
	}

	return ads, nil
}

// GetAdByID returns a single ad, or dao.ErrNotFound.
func (d *DAO) GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error) {
	const query = `SELECT ` + adColumns + ` FROM ads WHERE id = $1`

	ad, err := scanAd(d.runner().QueryRowContext(ctx, query, int64(id)))
	if err != nil {
		return domain.Ad{}, fmt.Errorf("unable to get ad: %w", err)
	}

	return ad, nil
}

// CreateAd inserts an ad and returns its generated identifier.
func (d *DAO) CreateAd(ctx context.Context, req adsdao.CreateAdRequest) (domain.AdID, error) {
	const query = `
		INSERT INTO ads (title, price_cents, photo_url, user_id)
		VALUES ($1, $2, $3, $4::uuid)
		RETURNING id`

	var id domain.AdID
	err := d.runner().QueryRowContext(ctx, query, req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return 0, fmt.Errorf("unable to create ad: %w", adsdao.ErrUnknownOwner)
		}

		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return id, nil
}

// DeleteAd deletes the ad if it belongs to ownerID, or returns dao.ErrNotFound.
func (d *DAO) DeleteAd(ctx context.Context, id domain.AdID, ownerID domain.UserID) error {
	const query = `DELETE FROM ads WHERE id = $1 AND user_id = $2::uuid`

	res, err := d.runner().ExecContext(ctx, query, int64(id), string(ownerID))
	if err != nil {
		return fmt.Errorf("unable to delete ad: %w", err)
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to delete ad: %w", err)
	}

	if deleted == 0 {
		return adsdao.ErrNotFound
	}

	return nil
}

// ClaimImport reserves an idempotency key, or returns the import that already used it.
//
// ON CONFLICT DO NOTHING relies on the primary key: when two imports race with the same key,
// the second INSERT waits for the first transaction, then either claims the key (first rolled
// back) or reads its committed result.
func (d *DAO) ClaimImport(ctx context.Context, ownerID domain.UserID, key, requestHash string) (adsdao.ImportRecord, bool, error) {
	const claim = `
		INSERT INTO ad_imports (user_id, idempotency_key, request_hash)
		VALUES ($1::uuid, $2, $3)
		ON CONFLICT (user_id, idempotency_key) DO NOTHING`

	res, err := d.runner().ExecContext(ctx, claim, string(ownerID), key, requestHash)
	if err != nil {
		return adsdao.ImportRecord{}, false, fmt.Errorf("unable to claim import: %w", err)
	}

	claimed, err := res.RowsAffected()
	if err != nil {
		return adsdao.ImportRecord{}, false, fmt.Errorf("unable to claim import: %w", err)
	}

	if claimed == 1 {
		return adsdao.ImportRecord{}, true, nil
	}

	const previous = `
		SELECT request_hash, COALESCE(ad_ids, '[]'::jsonb)
		FROM ad_imports
		WHERE user_id = $1::uuid AND idempotency_key = $2`

	var (
		record adsdao.ImportRecord
		rawIDs []byte
	)
	if err := d.runner().QueryRowContext(ctx, previous, string(ownerID), key).Scan(&record.RequestHash, &rawIDs); err != nil {
		return adsdao.ImportRecord{}, false, fmt.Errorf("unable to read previous import: %w", err)
	}

	if err := json.Unmarshal(rawIDs, &record.AdIDs); err != nil {
		return adsdao.ImportRecord{}, false, fmt.Errorf("unable to decode previous import: %w", err)
	}

	return record, false, nil
}

// CompleteImport stores the identifiers created by a claimed import.
func (d *DAO) CompleteImport(ctx context.Context, ownerID domain.UserID, key string, adIDs []domain.AdID) error {
	const query = `
		UPDATE ad_imports SET ad_ids = $3::jsonb
		WHERE user_id = $1::uuid AND idempotency_key = $2`

	rawIDs, err := json.Marshal(adIDs)
	if err != nil {
		return fmt.Errorf("unable to encode import result: %w", err)
	}

	if _, err := d.runner().ExecContext(ctx, query, string(ownerID), key, string(rawIDs)); err != nil {
		return fmt.Errorf("unable to complete import: %w", err)
	}

	return nil
}

func scanAd(row *sql.Row) (domain.Ad, error) {
	var ad domain.Ad
	if err := row.Scan(&ad.ID, &ad.Title, &ad.Price, &ad.PhotoURL, &ad.UserID, &ad.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Ad{}, adsdao.ErrNotFound
		}

		return domain.Ad{}, err
	}

	return ad, nil
}
