package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/sqldb"
	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
)

var (
	connection  = flag.String("connection", "postgresql://cj_license_manager:<managerpassword>@localhost:5432/cjlicenses", "specify a connection")
	runList     = flag.Bool("list", false, "List the current licenses")
	runListAll  = flag.Bool("listall", false, "List all fields for the current licenses")
	runAdd      = flag.Bool("add", false, "add a new license in the form <id> <vendor> <feature>")
	runRemove   = flag.String("remove", "", "remove an existing license by <id>")
	runShowLogs = flag.Bool("showlogs", false, "Show the logs from the license_state_log. You must specify a count to show")
	runFree     = flag.String("free", "", "Free a license by <id>")
)

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db, err := pgxpool.New(ctx, *connection)
	if err != nil {
		slog.Error("Error opening database: ", "error", err)
		return
	}
	defer db.Close()
	defer db.Reset() // If we don't do this, we seem to leak active connections.

	rows, err := db.Query(ctx, sqldb.QueryAllLicenses)
	if err != nil {
		slog.Error("Error querying licenses", "error", err)
		return
	}
	licenses, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*types.License, error) {
		l := &types.License{}
		err := row.Scan(&l.ID, &l.Vendor, &l.Feature, &l.Status, &l.LastUpdateTime, &l.UserNode, &l.UserProcess)
		return l, err
	})
	if err != nil {
		slog.Error("Error collectrows for licenses:", "error", err)
		return
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.Error("Error rows.Err for licenses:", "error", err)
		return
	}
	if *runList || *runListAll {
		for _, l := range licenses {
			if *runList {
				fmt.Printf("License: %v, %v, %v\n", l.ID, l.Vendor, l.Feature)
			}
			if *runListAll {
				var usernode, userprocess string
				if l.UserNode != nil {
					usernode = *l.UserNode
				} else {
					usernode = "<nil>"
				}
				if l.UserProcess != nil {
					userprocess = *l.UserProcess
				} else {
					userprocess = "<nil>"
				}
				fmt.Printf("License: %v, %v, %v, %v, %v, %v, %v\n", l.ID, l.Vendor, l.Feature, l.Status, l.LastUpdateTime, usernode, userprocess)
			}
		}
		return
	}
	if *runAdd {
		if flag.Arg(0) == "" || flag.Arg(1) == "" || flag.Arg(2) == "" {
			slog.Error("Arguments: -add <id> <vendor> <feature>")
			return
		}
		for _, l := range licenses {
			if l.ID == flag.Arg(0) {
				slog.Error("Unable to add a license with the same ID as another: ", "id", l.ID)
				return
			}
		}
		_, err = db.Exec(ctx, "insert into license_state (id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process) values ($1, $2, $3, $4, CURRENT_TIMESTAMP,$5, $6)",
			flag.Arg(0), flag.Arg(1), flag.Arg(2), sqldb.StateFree, nil, nil)
		if err != nil {
			slog.Error("Unable to insert into database:", "error", err)
			return
		}
		_, err = db.Exec(ctx, sqldb.NotifyLicenseState)
		if err != nil {
			slog.Error("Unable to notify licensestate change:", "error", err)
			return
		}
		fmt.Println("Added.")
		return
	}
	if *runRemove != "" {
		bFoundId := false
		for _, l := range licenses {
			if l.ID == *runRemove {
				bFoundId = true
			}
		}
		if !bFoundId {
			slog.Error("Can not find a license to remove named:", "input", *runRemove)
			return
		}
		_, err = db.Exec(ctx, "delete from license_state where id = $1",
			*runRemove)
		if err != nil {
			slog.Error("Unable to delete from database:", "error", err)
			return
		}
		_, err = db.Exec(ctx, sqldb.NotifyLicenseState)
		if err != nil {
			slog.Error("Unable to notify licensestate change:", "error", err)
			return
		}
		fmt.Println("Removed.")
		return
	}
	if *runShowLogs {
		if flag.Arg(0) == "" {
			slog.Error("You must specify a count of lines")
			return
		}
		rows, err := db.Query(ctx, "select license_id, node, ts, previous_state, current_state, reason, metadata from license_state_log order by ts desc limit "+flag.Arg(0))
		if err != nil {
			slog.Error("Error querying licenses:", "error", err)
			return
		}
		licenseLogs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*types.LicenseLog, error) {
			l := &types.LicenseLog{}
			err := row.Scan(&l.ID, &l.Node, &l.TimeStamp, &l.PreviousState, &l.CurrentState, &l.Reason, &l.Metadata)
			return l, err
		})
		if err != nil {
			slog.Error("Error collectrows for licenses:", "error", err)
			return
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			slog.Error("Error rows.Err for licenses:", "error", err)
		}
		for _, l := range licenseLogs {
			fmt.Printf("%v,%v,%v,%v,%v,%v,%v\n", l.ID, l.Node, l.TimeStamp, l.PreviousState, l.CurrentState, l.Reason, l.Metadata)
		}
		return
	}
	if *runFree != "" {
		bFoundId := false
		for _, l := range licenses {
			if l.ID == *runFree {
				bFoundId = true
			}
		}
		if !bFoundId {
			slog.Error("Can not find a license to remove named:", "command", *runFree)
			return
		}
		_, err = db.Exec(ctx, "update license_state set usage_state = $1, last_state_change = CURRENT_TIMESTAMP where id = $2",
			sqldb.StateFree, *runFree)
		if err != nil {
			slog.Error("Unable to update license_state:", "error", err)
		}
		_, err = db.Exec(ctx, sqldb.NotifyLicenseState)
		if err != nil {
			slog.Error("Unable to notify licensestate change:", "error", err)
		}
		fmt.Println("Freed.")
		return
	}
}
