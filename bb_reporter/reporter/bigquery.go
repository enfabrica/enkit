package reporter

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
)

// Inserter inserts a single row into a database.
type Inserter[T any] interface {
	Insert(context.Context, *T) error
}

// Inserter inserts multiple rows into a database.
type BatchInserter[T any] interface {
	BatchInsert(context.Context, []*T) error
}

// NewBigquery returns a BatchInserter that inserts into a specific table.
func NewBigquery[T any](ctx context.Context, projectID string, dataset string, table string) (BatchInserter[T], error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("while creating BigQuery client for project %q: %w", projectID, err)
	}
	return &BigqueryTable[T]{
		table: client.Dataset(dataset).Table(table).Inserter(),
	}, nil
}

// BigqueryTable is an Inserter and BatchInserter that inserts into a specific
// BigQuery table.
type BigqueryTable[T any] struct {
	table *bigquery.Inserter
}

func (b *BigqueryTable[T]) Insert(ctx context.Context, val *T) error {
	return b.table.Put(ctx, val)
}

func (b *BigqueryTable[T]) BatchInsert(ctx context.Context, vals []*T) error {
	return b.table.Put(ctx, vals)
}
