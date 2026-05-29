package report

import (
	"context"
	"database/sql"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetPayloadByUserID(ctx context.Context, userID string) ([]byte, error) {
	var payload string
	err := r.db.QueryRowContext(
		ctx,
		`SELECT report_payload FROM olap.mart_user_report FINAL WHERE user_id = ? ORDER BY updated_at DESC LIMIT 1`,
		userID,
	).Scan(&payload)
	return []byte(payload), err
}
