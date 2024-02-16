package main

import (
	"context"
	"flag"
	"fmt"
	"log"

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
	ctx := context.Background()
	db, err := pgxpool.New(ctx, *connection)
	if err != nil {
		log.Fatal("Error opening database: ", err)
	}
	rows, err := db.Query(ctx, sqldb.QueryAllLicenses)
	if err != nil {
		log.Fatal("Error querying licenses:", err)
	}
	licenses, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*types.License, error) {
		l := &types.License{}
		err := row.Scan(&l.ID, &l.Vendor, &l.Feature, &l.Status, &l.LastUpdateTime, &l.UserNode, &l.UserProcess)
		return l, err
	})
	if err != nil {
		log.Fatal("Error collectrows for licenses:", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Fatal("Error rows.Err for licenses:", err)
	}
	if *runList || *runListAll {
		for _, l := range licenses {
			if *runList {
				fmt.Printf("License: %v, %v, %v\n", l.ID, l.Vendor, l.Feature)
			}
			if *runListAll {
				fmt.Printf("License: %v, %v, %v, %v, %v, %v, %v\n", l.ID, l.Vendor, l.Feature, l.Status, l.LastUpdateTime, l.UserNode, l.UserProcess)
			}
		}
		return
	}
	if *runAdd {
		if flag.Arg(0) == "" || flag.Arg(1) == "" || flag.Arg(2) == "" {
			log.Fatal("Arguments: -add <id> <vendor> <feature>")
		}
		for _, l := range licenses {
			if l.ID == flag.Arg(0) {
				log.Fatal("Unable to add a license with the same ID as another: ", l.ID)
			}
		}
		_, err = db.Exec(ctx, "insert into license_state (id, vendor, feature, usage_state, last_state_change, reserved_by_node, used_by_process) values ($1, $2, $3, $4, CURRENT_TIMESTAMP,$5, $6)",
			flag.Arg(0), flag.Arg(1), flag.Arg(2), sqldb.StateFree, nil, nil)
		if err != nil {
			log.Fatal("Unable to insert into database:", err)
		}
		_, err = db.Exec(ctx, sqldb.NotifyLicenseState)
		if err != nil {
			log.Fatal("Unable to notify licensestate change:", err)
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
			log.Fatal("Can not find a license to remove named:", *runRemove)
		}
		_, err = db.Exec(ctx, "delete from license_state where id = $1",
			*runRemove)
		if err != nil {
			log.Fatal("Unable to delete from database:", err)
		}
		_, err = db.Exec(ctx, sqldb.NotifyLicenseState)
		if err != nil {
			log.Fatal("Unable to notify licensestate change:", err)
		}
		fmt.Println("Removed.")
		return
	}
	if *runShowLogs {
		if flag.Arg(0) == "" {
			log.Fatal("You must specify a count of lines")
		}
		rows, err := db.Query(ctx, "select license_id, node, ts, previous_state, current_state, reason, metadata from license_state_log order by ts desc limit "+flag.Arg(0))
		if err != nil {
			log.Fatal("Error querying licenses:", err)
		}
		licenseLogs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*types.LicenseLog, error) {
			l := &types.LicenseLog{}
			err := row.Scan(&l.ID, &l.Node, &l.TimeStamp, &l.PreviousState, &l.CurrentState, &l.Reason, &l.Metadata)
			return l, err
		})
		if err != nil {
			log.Fatal("Error collectrows for licenses:", err)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			log.Fatal("Error rows.Err for licenses:", err)
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
			log.Fatal("Can not find a license to remove named:", *runFree)
		}
		_, err = db.Exec(ctx, "update license_state set usage_state = $1, last_state_change = CURRENT_TIMESTAMP where id = $2",
			sqldb.StateFree, *runFree)
		if err != nil {
			log.Fatal("Unable to update license_state:", err)
		}
		_, err = db.Exec(ctx, sqldb.NotifyLicenseState)
		if err != nil {
			log.Fatal("Unable to notify licensestate change:", err)
		}
		fmt.Println("Freed.")
		return
	}
}
