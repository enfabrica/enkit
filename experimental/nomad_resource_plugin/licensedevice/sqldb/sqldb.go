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
	queryLocalLicenses    = "SELECT id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process FROM license_state WHERE usage_state = 'IN_USE' AND reserved_by_node = $1"
	querySingleLicense    = "SELECT id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process FROM license_state WHERE id = $1"
	updateLicenseState    = "UPDATE license_state SET usage_state = $2, last_state_change = $3, reserved_by_node = $4, used_by_process = $5 WHERE id = $1"
	appendLicenseStateLog = "INSERT INTO license_state_log (license_id, node, ts, previous_state, current_state, reason, metadata) VALUES ($1, $2, $3, $4, $5, $6, $7)"
	listenLicenseState    = "LISTEN license_state_update_channel"
	notifyLicenseState    = "NOTIFY license_state_update_channel"

	stateFree     = "FREE"
	stateReserved = "RESERVED"
	stateInUse    = "IN_USE"
)

type Table struct {
	db        *pgxpool.Pool
	tableName string
	nodeID    string
}

func OpenTable(ctx context.Context, connStr string, table string, nodeID string) (*Table, error) {
	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to DB: %w", err)
	}
	return &Table{
		db:        db,
		tableName: table,
		nodeID:    nodeID,
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
		err := row.Scan(&l.ID, &l.Vendor, &l.Feature, &l.Status, &l.LastUpdateTime, &l.UserNode, &l.UserProcess)
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
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start DB transaction: %w", err)
	}

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
			}
			return
		}

		if err := tx.Commit(ctx); err != nil {
			retErr = fmt.Errorf("failed to commit DB changes: %w", err)
			ret = nil
			// TODO(scott): log/metric
		}
	}()

	licenses := []*types.License{}
	for _, id := range licenseIDs {
		licenses = append(licenses, &types.License{
			ID:       id,
			Status:   "RESERVED",
			UserNode: &node,
		})
	}

	return t.updateLicenses(ctx, tx, licenses, "Reserve() called on device plugin")
}

func (t *Table) UpdateInUse(ctx context.Context, licenses []*types.License) (retErr error) {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start DB transaction: %w", err)
	}

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
			}
			return
		}

		if err := tx.Commit(ctx); err != nil {
			retErr = fmt.Errorf("failed to commit DB changes: %w", err)
			// TODO(scott): log/metric
		}
	}()

	localLicenses, err := t.getLicenses(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to get currently-used licenses for %q: %w", t.nodeID, err)
	}

	// Every license returned as IN_USE by the DB but not mentioned in the
	// supplied licenses is no longer used, and needs to be freed.
nextLicense:
	for _, localLicense := range localLicenses {
		for _, l := range licenses {
			if localLicense.ID == l.ID {
				continue nextLicense
			}
		}
		localLicense.LastUpdateTime = time.Now()
		localLicense.UserNode = nil
		localLicense.UserProcess = nil
		localLicense.Status = "FREE"
		licenses = append(licenses, localLicense)
	}

	_, err = t.updateLicenses(ctx, tx, licenses, "TODO: plumb reason")
	if err != nil {
		return fmt.Errorf("failed to update license status: %w", err)
	}

	return nil
}

func (t *Table) getLicenses(ctx context.Context, tx pgx.Tx) ([]*types.License, error) {
	rows, err := tx.Query(ctx, queryLocalLicenses, t.nodeID)
	if err != nil {
		return nil, fmt.Errorf("DB read for local licenses failed: %w", err)
	}
	defer rows.Close()
	licenses, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*types.License, error) {
		l := &types.License{}
		err := row.Scan(&l.ID, &l.Vendor, &l.Feature, &l.Status, &l.LastUpdateTime, &l.UserNode, &l.UserProcess)
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

func (t *Table) updateLicenses(ctx context.Context, tx pgx.Tx, licenses []*types.License, reason string) ([]*types.License, error) {
	ret := []*types.License{}

	txTime := time.Now()

nextLicense:
	for _, license := range licenses {
		row := tx.QueryRow(ctx, querySingleLicense, license.ID)
		dbLicense := &types.License{}
		if err := row.Scan(&dbLicense.ID, &dbLicense.Vendor, &dbLicense.Feature, &dbLicense.Status, &dbLicense.LastUpdateTime, &dbLicense.UserNode, &dbLicense.UserProcess); err != nil {
			return nil, fmt.Errorf("failed to get current state of license %q: %w", license.ID, err)
		}

		if dbLicense.Status == license.Status {
			continue nextLicense
		}

		tag, err := tx.Exec(
			ctx,
			updateLicenseState,
			license.ID,
			license.Status,
			txTime,
			license.UserNode,
			license.UserProcess,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update row for license %q: %w", license.ID, err)
		}
		if tag.RowsAffected() != 1 {
			return nil, fmt.Errorf("license reserve affected %d rows; expected exactly one row affected", tag.RowsAffected)
		}

		_, err = tx.Exec(
			ctx,
			appendLicenseStateLog,
			license.ID,
			t.nodeID,
			txTime,
			dbLicense.Status,
			license.Status,
			reason,
			map[string]interface{}{},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update license_state_log for license %q: %w", license.ID, err)
		}

		_, err = tx.Exec(ctx, notifyLicenseState)
		if err != nil {
			return nil, fmt.Errorf("failed to notify other plugins of update for license %q: %w", license.ID, err)
		}

		// We could read back from the DB; this is equivalent
		license.LastUpdateTime = txTime
		ret = append(ret, license)
	}
	fmt.Println("all updates successful")
	return ret, nil
}

func (t *Table) Chan(ctx context.Context) chan struct{} {
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
