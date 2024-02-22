package sqldb

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
)

const (
	QueryAllLicenses      = "SELECT id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process FROM license_state"
	queryLocalLicenses    = "SELECT id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process FROM license_state WHERE usage_state = 'IN_USE' AND reserved_by_node = $1"
	querySingleLicense    = "SELECT id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process FROM license_state WHERE id = $1"
	updateLicenseState    = "UPDATE license_state SET usage_state = $2, last_state_change = $3, reserved_by_node = $4, used_by_process = $5 WHERE id = $1"
	appendLicenseStateLog = "INSERT INTO license_state_log (license_id, node, ts, previous_state, current_state, reason, metadata) VALUES ($1, $2, $3, $4, $5, $6, $7)"
	listenLicenseState    = "LISTEN license_state_update_channel"
	NotifyLicenseState    = "NOTIFY license_state_update_channel"

	StateFree     = "FREE"
	stateReserved = "RESERVED"
	stateInUse    = "IN_USE"
)

var (
	metricGetCurrentDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "licensedevice",
		Subsystem: "sqldb",
		Name:      "get_current_duration_seconds",
		Help:      "GetCurrent execution time",
	})
	metricMyLicenses = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "licensedevice",
		Subsystem: "sqldb",
		Name:      "my_licenses",
		Help:      "How many licenses do I currently have",
	})
	metricSqlCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "licensedevice",
		Subsystem: "sqldb",
		Name:      "results",
		Help:      "The number of times sql has succeeded or errored in various sections of the code",
	},
		[]string{
			"location",
			"outcome",
		})
)

type Table struct {
	db        *pgxpool.Pool
	tableName string
	nodeID    string
	Log       hclog.Logger
}

func OpenTable(ctx context.Context, connStr string, table string, nodeID string, log hclog.Logger) (*Table, error) {
	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		metricSqlCounter.WithLabelValues("OpenTable", "error_open_table").Inc()
		return nil, fmt.Errorf("failed to open connection to DB: %w", err)
	}
	metricSqlCounter.WithLabelValues("OpenTable", "ok").Inc()
	return &Table{
		db:        db,
		tableName: table,
		nodeID:    nodeID,
		Log:       log,
	}, nil
}

func (t *Table) GetCurrent(ctx context.Context) ([]*types.License, error) {
	startTime := time.Now()
	defer metricGetCurrentDuration.Observe(float64(time.Now().Sub(startTime).Seconds()))
	rows, err := t.db.Query(ctx, QueryAllLicenses)
	if err != nil {
		metricSqlCounter.WithLabelValues("GetCurrent", "error_query_all_licenses").Inc()
		return nil, fmt.Errorf("DB read for all licenses failed: %w", err)
	}
	defer rows.Close()

	licenses, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*types.License, error) {
		l := &types.License{}
		err := row.Scan(&l.ID, &l.Vendor, &l.Feature, &l.Status, &l.LastUpdateTime, &l.UserNode, &l.UserProcess)
		return l, err
	})
	if err != nil {
		metricSqlCounter.WithLabelValues("GetCurrent", "error_collect_rows").Inc()
		return nil, fmt.Errorf("error translating to types.License from DB row: %w", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		metricSqlCounter.WithLabelValues("GetCurrent", "error_close").Inc()
		return nil, fmt.Errorf("DB read for all licenses failed after Close: %w", err)
	}
	metricSqlCounter.WithLabelValues("GetCurrent", "ok").Inc()
	return licenses, nil
}

func (t *Table) Reserve(ctx context.Context, licenseIDs []string, node string) (ret []*types.License, retErr error) {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		metricSqlCounter.WithLabelValues("Reserve", "error_db_begin").Inc()
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
				metricSqlCounter.WithLabelValues("Reserve", "error_rollback").Inc()
				t.Log.Error("Error, failed to rollback", "original_error", retErr, "rollback_error", err)
			}
			return
		}

		if err := tx.Commit(ctx); err != nil {
			retErr = fmt.Errorf("failed to commit DB changes: %w", err)
			metricSqlCounter.WithLabelValues("Reserve", "error_commit").Inc()
			t.Log.Error("Error, failed to commit", "commit_error", retErr)
			ret = nil
		} else {
			metricSqlCounter.WithLabelValues("Reserve", "ok").Inc()
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
		metricSqlCounter.WithLabelValues("UpdateInUse", "error_begin").Inc()
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
				metricSqlCounter.WithLabelValues("UpdateInUse", "error_rollback").Inc()
				t.Log.Error("Error, failed to rollback", "original_error", retErr, "rollback_error", err)
			}
			return
		}

		if err := tx.Commit(ctx); err != nil {
			retErr = fmt.Errorf("failed to commit DB changes: %w", err)
			metricSqlCounter.WithLabelValues("UpdateInUse", "error_commit").Inc()
			t.Log.Error("Error, failed to commit in UpdateInUse", "commit_error", retErr)
		}
	}()

	localLicenses, err := t.getLicenses(ctx, tx)
	if err != nil {
		metricSqlCounter.WithLabelValues("UpdateInUse", "error_get_licenses").Inc()
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

	_, err = t.updateLicenses(ctx, tx, licenses, "detected in scan")
	if err != nil {
		metricSqlCounter.WithLabelValues("UpdateInUse", "error_update_licenses").Inc()
		return fmt.Errorf("failed to update license status: %w", err)
	}
	metricSqlCounter.WithLabelValues("UpdateInUse", "ok").Inc()
	return nil
}

func (t *Table) getLicenses(ctx context.Context, tx pgx.Tx) ([]*types.License, error) {
	rows, err := tx.Query(ctx, queryLocalLicenses, t.nodeID)
	if err != nil {
		metricSqlCounter.WithLabelValues("getLicenses", "error_query").Inc()
		return nil, fmt.Errorf("DB read for local licenses failed: %w", err)
	}
	defer rows.Close()
	licenses, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*types.License, error) {
		l := &types.License{}
		err := row.Scan(&l.ID, &l.Vendor, &l.Feature, &l.Status, &l.LastUpdateTime, &l.UserNode, &l.UserProcess)
		return l, err
	})
	metricMyLicenses.Set(float64(len(licenses)))
	if err != nil {
		metricSqlCounter.WithLabelValues("getLicenses", "error_collect_rows").Inc()
		return nil, fmt.Errorf("error translating to types.License from DB row: %w", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		metricSqlCounter.WithLabelValues("getLicenses", "error_close").Inc()
		return nil, fmt.Errorf("DB read for all licenses failed after Close: %w", err)
	}
	metricSqlCounter.WithLabelValues("getLicenses", "ok").Inc()
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
			metricSqlCounter.WithLabelValues("updateLicenses", "error_scan").Inc()
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
			metricSqlCounter.WithLabelValues("updateLicenses", "error_update_license_state").Inc()
			return nil, fmt.Errorf("failed to update row for license %q: %w", license.ID, err)
		}
		if tag.RowsAffected() != 1 {
			metricSqlCounter.WithLabelValues("updateLicenses", "error_too_many_rows_affected").Inc()
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
			metricSqlCounter.WithLabelValues("updateLicenses", "error_exec_append_license_state_log").Inc()
			return nil, fmt.Errorf("failed to update license_state_log for license %q: %w", license.ID, err)
		}

		_, err = tx.Exec(ctx, NotifyLicenseState)
		if err != nil {
			metricSqlCounter.WithLabelValues("updateLicenses", "error_license_state_notify").Inc()
			return nil, fmt.Errorf("failed to notify other plugins of update for license %q: %w", license.ID, err)
		}

		// We could read back from the DB; this is equivalent
		license.LastUpdateTime = txTime
		ret = append(ret, license)
	}
	fmt.Println("all updates successful")
	metricSqlCounter.WithLabelValues("updateLicenses", "ok").Inc()
	return ret, nil
}

func (t *Table) Chan(ctx context.Context) chan struct{} {
	c := make(chan struct{})

	conn, err := t.db.Acquire(ctx)
	if err != nil {
		t.Log.Error("Error, failed to db Acquire", "db_error", err)
		metricSqlCounter.WithLabelValues("Chan", "error_acquire_db").Inc()
		close(c)
	}

	_, err = conn.Exec(ctx, listenLicenseState)
	if err != nil {
		t.Log.Error("Error, failed to listenLicenseState", "db_error", err)
		metricSqlCounter.WithLabelValues("Chan", "error_exec_listen_license_state").Inc()
		close(c)
	}

	go func() {
		defer conn.Release()
		for {
			_, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				t.Log.Error("Error, failed to WaitForNotification", "db_error", err)
				metricSqlCounter.WithLabelValues("Chan", "error_wait_for_notification").Inc()
				return
			}
			c <- struct{}{}
		}
	}()
	metricSqlCounter.WithLabelValues("Chan", "ok").Inc()
	return c
}
