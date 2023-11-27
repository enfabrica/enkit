package sqldb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
)

const (
	queryAllLicenses      = "SELECT id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process FROM license_state"
	querySingleLicense    = "SELECT id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process FROM license_state WHERE id = $1"
	updateLicenseState    = "UPDATE license_state SET usage_state = $2, last_state_change = $3, reserved_by_node = $4, used_by_process = $5 WHERE id = $1"
	appendLicenseStateLog = "INSERT INTO license_state_log VALUES ($1, $2, $3, $4, $5, $6, $7)"
	listenLicenseState    = "LISTEN license_state_update_channel"
	notifyLicenseState    = "NOTIFY license_state_update_channel"

	stateFree     = "FREE"
	stateReserved = "RESERVED"
	stateInUse    = "IN_USE"
)

type Table struct {
	db        *pgxpool.Pool
	tableName string
}

func OpenTable(ctx context.Context, connStr string, table string) (*Table, error) {
	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to DB: %w", err)
	}
	return &Table{
		db:        db,
		tableName: table,
	}, nil
}

func (t *Table) GetCurrent(ctx context.Context) ([]*types.License, error) {
	rows, err := t.db.Query(ctx, queryAllLicenses)
	if err != nil {
		return nil, fmt.Errorf("DB read for all licenses failed: %w", err)
	}
	defer rows.Close()

	licenses, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*types.License, error) {
		l := &types.License{}
		err := row.Scan(l.ID, l.Vendor, l.Feature, l.Status, l.LastUpdateTime, l.UserNode, l.UserProcess)
		return l, err
	})
	if err != nil {
		return nil, fmt.Errorf("error translating to types.License from DB row: %w", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("DB read for all licenses failed after Close: %w", err)
	}
	return licenses, nil
}

func (t *Table) Reserve(ctx context.Context, licenseIDs []string, node string) (ret []*types.License, retErr error) {
	return t.updateLicenses(ctx, licenseIDs, node, nil, stateReserved, "Reserve() called on device plugin")
}

func (t *Table) Use(ctx context.Context, licenseID string, node string, user string) error {
	_, err := t.updateLicenses(ctx, []string{licenseID}, node, &user, stateInUse, "Container mounted license handle file")
	return err
}

func (t *Table) Free(ctx context.Context, licenseID string, node string) error {
	_, err := t.updateLicenses(ctx, []string{licenseID}, node, nil, stateFree, "License handle file was unmounted")
	return err
}

func (t *Table) updateLicenses(ctx context.Context, licenseIDs []string, node string, user *string, state string, reason string) (ret []*types.License, retErr error) {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start DB transaction: %w", err)
	}

	txTime := time.Now()

	defer func() {
		// Handle the transaction commit/rollback here in one place, by hooking the
		// end of the function and checking the error being returned. An error means
		// roll back; no error means commit.

		// If rolling back, this could be due to a cancelled context. In this case,
		// we can't use the context to rollback the transaction, so make a new
		// ephemeral one from the background context.
		shortCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if retErr != nil {
			if err := tx.Rollback(shortCtx); err != nil {
				// TODO(scott): log/metric
				return
			}
		}

		if err := tx.Commit(ctx); err != nil {
			retErr = fmt.Errorf("failed to commit DB changes: %w", err)
			ret = nil
			// TODO(scott): log/metric
		}
	}()

	for _, id := range licenseIDs {
		row := tx.QueryRow(ctx, querySingleLicense, id)
		license := &types.License{}
		if err := row.Scan(license.ID, license.Vendor, license.Feature, license.Status, license.LastUpdateTime, license.UserNode, license.UserProcess); err != nil {
			return nil, fmt.Errorf("failed to get current state of license %q: %w", id, err)
		}

		tag, err := tx.Exec(
			ctx,
			updateLicenseState,
			id,
			stateReserved,
			txTime,
			node,
			user,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update row for license %q: %w", id, err)
		}
		if tag.RowsAffected() != 1 {
			return nil, fmt.Errorf("license reserve affected %d rows; expected exactly one row affected", tag.RowsAffected)
		}

		_, err = tx.Exec(
			ctx,
			appendLicenseStateLog,
			id,
			node,
			txTime,
			license.Status,
			state,
			reason,
			map[string]interface{}{},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update license_state_log for license %q: %w", id, err)
		}

		_, err = tx.Exec(ctx, notifyLicenseState)
		if err != nil {
			return nil, fmt.Errorf("failed to notify other plugins of update for license %q: %w", id, err)
		}

		// We could read back from the DB; this is equivalent
		license.Status = stateReserved
		license.LastUpdateTime = txTime
		license.UserNode = &node
		license.UserProcess = nil
		ret = append(ret, license)
	}
	return
}

func (t *Table) Chan(ctx context.Context) <-chan struct{} {
	c := make(chan struct{})

	conn, err := t.db.Acquire(ctx)
	if err != nil {
		// TODO(scott): error + metric
		close(c)
	}

	_, err = conn.Exec(ctx, listenLicenseState)
	if err != nil {
		// TODO(scott): error + metric
		close(c)
	}

	go func() {
		defer conn.Release()
		for {
			_, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				// TODO(scott): error + metric
				return
			}
			c <- struct{}{}
		}
	}()

	return c
}
