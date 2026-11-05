package store

import (
	"context"
	"database/sql"
)

// DropHousehold deletes a household and everything in it, e.g. one a
// restore didn't finish, and returns the files to remove.
func (s *Store) DropHousehold(ctx context.Context, hh int64) ([]string, error) {
	var files []string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var err error
		files, err = deleteHousehold(ctx, tx, hh)
		return err
	})
	return files, err
}

// RestoreTimes gives a restored thing the times it had: when it was added,
// last changed and archived.
func (s *Store) RestoreTimes(ctx context.Context, hh, thing int64, created, updated string, archived *string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE things SET created_at = COALESCE(NULLIF(?, ''), created_at),
		updated_at = COALESCE(NULLIF(?, ''), updated_at), archived_at = ? WHERE id = ? AND household_id = ?`,
		created, updated, archived, thing, hh)
	return err
}

// RestoreMaintenanceTimes gives a restored maintenance entry the times it had.
func (s *Store) RestoreMaintenanceTimes(ctx context.Context, id int64, created, updated string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE maintenance SET created_at = COALESCE(NULLIF(?, ''), created_at),
		updated_at = COALESCE(NULLIF(?, ''), updated_at) WHERE id = ?`, created, updated, id)
	return err
}

// KeepTimes gives a thing brought over from elsewhere the times it had
// there. Empty ones are left as they are.
func (s *Store) KeepTimes(ctx context.Context, hh, thing int64, created, updated string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE things SET created_at = COALESCE(NULLIF(?, ''), created_at),
		updated_at = COALESCE(NULLIF(?, ''), updated_at) WHERE id = ? AND household_id = ?`,
		created, updated, thing, hh)
	return err
}
