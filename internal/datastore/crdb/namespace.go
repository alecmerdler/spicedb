package crdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	v0 "github.com/authzed/authzed-go/proto/authzed/api/v0"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/proto"

	"github.com/authzed/spicedb/internal/datastore"
)

const (
	errUnableToWriteConfig    = "unable to write namespace config: %w"
	errUnableToReadConfig     = "unable to read namespace config: %w"
	errUnableToDeleteConfig   = "unable to delete namespace config: %w"
	errUnableToListNamespaces = "unable to list namespaces: %w"
)

type updateIntention bool

var (
	forUpdate updateIntention = true
	readOnly  updateIntention = false
)

var (
	upsertNamespaceSuffix = fmt.Sprintf(
		"ON CONFLICT (%s) DO UPDATE SET %s = excluded.%s %s",
		colNamespace,
		colConfig,
		colConfig,
		queryReturningTimestamp,
	)
	queryWriteNamespace = psql.Insert(tableNamespace).Columns(
		colNamespace,
		colConfig,
	).Suffix(upsertNamespaceSuffix)

	queryReadNamespace = psql.Select(colConfig, colTimestamp).From(tableNamespace)

	queryDeleteNamespace = psql.Delete(tableNamespace)
)

func (cds *crdbDatastore) WriteNamespace(ctx context.Context, newConfig *v0.NamespaceDefinition) (datastore.Revision, error) {
	var hlcNow decimal.Decimal
	if err := cds.execute(ctx, cds.conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		serialized, err := proto.Marshal(newConfig)
		if err != nil {
			return err
		}
		writeSQL, writeArgs, err := queryWriteNamespace.
			Values(newConfig.Name, serialized).
			ToSql()
		if err != nil{
			return err
		}
		return cds.conn.QueryRow(
			datastore.SeparateContextWithTracing(ctx), writeSQL, writeArgs...,
		).Scan(&hlcNow)
	}); err != nil {
		return datastore.NoRevision, fmt.Errorf(errUnableToWriteConfig, err)
	}

	return hlcNow, nil
}

func (cds *crdbDatastore) ReadNamespace(ctx context.Context, nsName string) (*v0.NamespaceDefinition, datastore.Revision, error) {
	ctx = datastore.SeparateContextWithTracing(ctx)

	var config *v0.NamespaceDefinition
	var timestamp time.Time
	if err := cds.execute(ctx, cds.conn, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		config, timestamp, err = loadNamespace(ctx, tx, nsName, readOnly)
		return err
	}); err != nil {
		if errors.As(err, &datastore.ErrNamespaceNotFound{}) {
			return nil, datastore.NoRevision, err
		}
		return nil, datastore.NoRevision, fmt.Errorf(errUnableToReadConfig, err)
	}

	return config, revisionFromTimestamp(timestamp), nil
}

func (cds *crdbDatastore) DeleteNamespace(ctx context.Context, nsName string) (datastore.Revision, error) {
	ctx = datastore.SeparateContextWithTracing(ctx)

	var timestamp time.Time
	if err := cds.execute(ctx, cds.conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		_, timestamp, err = loadNamespace(ctx, tx, nsName, readOnly)
		delSQL, delArgs, err := queryDeleteNamespace.
			Where(sq.Eq{colNamespace: nsName, colTimestamp: timestamp}).
			ToSql()
		if err != nil {
			return err
		}

		deletedNSResult, err := tx.Exec(ctx, delSQL, delArgs...)
		if err != nil {
			return err
		}
		numDeleted := deletedNSResult.RowsAffected()
		if numDeleted != 1 {
			log.Warn().Int64("numDeleted", numDeleted).Msg("deleted wrong number of namespaces")
		}

		deleteTupleSQL, deleteTupleArgs, err := queryDeleteTuples.
			Where(sq.Eq{colNamespace: nsName}).
			ToSql()
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, deleteTupleSQL, deleteTupleArgs...)
		return err
	}); err != nil {
		if errors.As(err, &datastore.ErrNamespaceNotFound{}) {
			return datastore.NoRevision, err
		}
		return datastore.NoRevision, fmt.Errorf(errUnableToDeleteConfig, err)
	}

	return revisionFromTimestamp(timestamp), nil
}

func loadNamespace(ctx context.Context, tx pgx.Tx, nsName string, forUpdate updateIntention) (*v0.NamespaceDefinition, time.Time, error) {
	query := queryReadNamespace.Where(sq.Eq{colNamespace: nsName})

	if forUpdate {
		query.Suffix("FOR UPDATE")
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, time.Time{}, err
	}

	var config []byte
	var timestamp time.Time
	if err := tx.QueryRow(ctx, sql, args...).Scan(&config, &timestamp); err != nil {
		if err == pgx.ErrNoRows {
			err = datastore.NewNamespaceNotFoundErr(nsName)
		}
		return nil, time.Time{}, err
	}

	loaded := &v0.NamespaceDefinition{}
	err = proto.Unmarshal(config, loaded)
	if err != nil {
		return nil, time.Time{}, err
	}

	return loaded, timestamp, nil
}

func (cds *crdbDatastore) ListNamespaces(ctx context.Context) ([]*v0.NamespaceDefinition, error) {
	ctx = datastore.SeparateContextWithTracing(ctx)

	var nsDefs []*v0.NamespaceDefinition
	err := cds.execute(ctx, cds.conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		query := queryReadNamespace

		sql, args, err := query.ToSql()
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, sql, args...)
		defer rows.Close()

		for rows.Next() {
			var config []byte
			var timestamp time.Time
			if err := rows.Scan(&config, &timestamp); err != nil {
				return fmt.Errorf(errUnableToListNamespaces, err)
			}

			var loaded v0.NamespaceDefinition
			if err := proto.Unmarshal(config, &loaded); err != nil {
				return fmt.Errorf(errUnableToReadConfig, err)
			}

			nsDefs = append(nsDefs, &loaded)
		}
		return nil
	})

	return nsDefs, err
}
