package sqldb

import (
	"context"
	"fmt"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
)

type Table struct{}

func OpenTable(ctx context.Context, connStr string, table string) (*Table, error) {
	return nil, fmt.Errorf("OpenTable unimplemented")
}

func (t *Table) GetCurrent(ctx context.Context) ([]*types.License, error) {
	return nil, fmt.Errorf("GetCurrent unimplemented")
}

func (t *Table) Reserve(ctx context.Context, licenseID string, node string, user string) error {
	return fmt.Errorf("Reserve unimplemented")
}

func (t *Table) Use(ctx context.Context, licenseID string, node string, user string) error {
	return fmt.Errorf("Use unimplemented")
}

func (t *Table) Free(ctx context.Context, licenseID string, node string) error {
	return fmt.Errorf("Free unimplemented")
}

func (t *Table) Chan(ctx context.Context) <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}
