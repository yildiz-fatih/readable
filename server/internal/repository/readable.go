package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yildiz-fatih/readable/server/internal/models"
)

var ErrReadableNotFound = errors.New("readable not found")

type ReadableRepository struct {
	dbPool *pgxpool.Pool
}

func NewReadableRepository(dbPool *pgxpool.Pool) *ReadableRepository {
	return &ReadableRepository{dbPool: dbPool}
}

func (r *ReadableRepository) Get(ctx context.Context, id int) (*models.Readable, error) {
	query := "SELECT id, status, format, created FROM readables WHERE id = $1"

	var readable models.Readable
	err := r.dbPool.QueryRow(ctx, query, id).Scan(&readable.ID, &readable.Status, &readable.Format, &readable.Created)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReadableNotFound
		}

		return nil, err
	}

	return &readable, nil
}

func (r *ReadableRepository) CreateTx(ctx context.Context, transaction pgx.Tx, id int, format models.ReadableFormat) (*models.Readable, error) {
	query := "INSERT INTO readables (id, format) VALUES ($1, $2) RETURNING id, status, format, created"

	var readable models.Readable
	err := transaction.QueryRow(ctx, query, id, format).Scan(&readable.ID, &readable.Status, &readable.Format, &readable.Created)
	if err != nil {
		return nil, err
	}

	return &readable, nil
}

func (r *ReadableRepository) UpdateStatus(ctx context.Context, id int, status models.ReadableStatus) error {
	query := "UPDATE readables SET status = $1 WHERE id = $2"

	_, err := r.dbPool.Exec(ctx, query, status, id)
	if err != nil {
		return err
	}

	return nil
}
