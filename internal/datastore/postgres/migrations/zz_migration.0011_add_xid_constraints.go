package migrations

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
)

const (
	getNSConfigPkeyName = `
	SELECT constraint_name FROM information_schema.table_constraints
		WHERE table_schema = 'public'
      	AND table_name = 'namespace_config'
      	AND constraint_type = 'PRIMARY KEY';`

	dropNSConfigIDPkey = "ALTER TABLE namespace_config DROP CONSTRAINT %s;"
)

var addXIDConstraints = []string{
	`ALTER TABLE relation_tuple_transaction
		DROP CONSTRAINT pk_rttx,
		ADD CONSTRAINT pk_rttx PRIMARY KEY USING INDEX ix_rttx_pk;`,
	`ALTER TABLE namespace_config
		ADD CONSTRAINT pk_namespace_config PRIMARY KEY USING INDEX ix_namespace_config_pk,
		ADD CONSTRAINT uq_namespace_living_xid UNIQUE USING INDEX ix_namespace_config_living;`,
	`ALTER TABLE relation_tuple
		DROP CONSTRAINT pk_relation_tuple,
		ADD CONSTRAINT pk_relation_tuple PRIMARY KEY USING INDEX ix_relation_tuple_pk,
		ADD CONSTRAINT uq_relation_tuple_living_xid UNIQUE USING INDEX ix_relation_tuple_living;`,
}

func init() {
	if err := DatabaseMigrations.Register("add-xid-constraints", "add-xid-indices",
		noNonatomicMigration,
		func(ctx context.Context, tx pgx.Tx) error {
			var constraintName string
			if err := tx.QueryRow(ctx, getNSConfigPkeyName).Scan(&constraintName); err != nil {
				return err
			}

			if _, err := tx.Exec(ctx, fmt.Sprintf(dropNSConfigIDPkey, constraintName)); err != nil {
				return err
			}

			for _, stmt := range addXIDConstraints {
				if _, err := tx.Exec(ctx, stmt); err != nil {
					return err
				}
			}

			return nil
		}); err != nil {
		panic("failed to register migration: " + err.Error())
	}
}