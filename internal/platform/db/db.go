// Package db owns connection pools and transaction-local RLS contexts.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	return pgxpool.NewWithConfig(ctx, config)
}

func WithSystemTx(ctx context.Context, p *pgxpool.Pool, fn func(pgx.Tx) error) error {
	return withTx(ctx, p, pgx.TxOptions{}, "system", "", "", fn)
}
func WithSellerTx(ctx context.Context, p *pgxpool.Pool, sellerID uuid.UUID, fn func(pgx.Tx) error) error {
	return withTx(ctx, p, pgx.TxOptions{}, "seller", sellerID.String(), "", fn)
}
func WithBuyerTx(ctx context.Context, p *pgxpool.Pool, buyerID uuid.UUID, fn func(pgx.Tx) error) error {
	return withTx(ctx, p, pgx.TxOptions{}, "buyer", "", buyerID.String(), fn)
}
func WithPublicTx(ctx context.Context, p *pgxpool.Pool, fn func(pgx.Tx) error) error {
	return withTx(ctx, p, pgx.TxOptions{AccessMode: pgx.ReadOnly}, "public", "", "", fn)
}

type beginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

func withTx(ctx context.Context, p beginner, options pgx.TxOptions, role, seller, buyer string, fn func(pgx.Tx) error) (err error) {
	tx, err := p.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback also runs during panic, using a live context even when the caller cancelled.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rollbackErr := tx.Rollback(cleanup)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
	}()
	// Clear unrelated identities as well, so an inherited session setting cannot widen access.
	if _, err = tx.Exec(ctx, `SELECT set_config('app.role', $1, true), set_config('app.seller_id', $2, true), set_config('app.buyer_id', $3, true)`, role, seller, buyer); err != nil {
		return fmt.Errorf("set transaction context: %w", err)
	}
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
