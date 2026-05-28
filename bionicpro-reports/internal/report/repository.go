package report

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetPayloadByUserID(ctx context.Context, userID string) ([]byte, error) {
	var payload []byte
	err := r.pool.QueryRow(
		ctx,
		`SELECT report_payload::text FROM olap.mart_user_report WHERE user_id = $1`,
		userID,
	).Scan(&payload)
	return payload, err
}
