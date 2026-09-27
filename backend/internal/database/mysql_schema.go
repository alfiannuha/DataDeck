package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// MySQL system databases (information_schema, mysql, performance_schema, sys)
// are excluded from the explorer. MySQL has no schema level below the database,
// so introspection is scoped to the *connected* database only — system schemas
// are therefore never enumerated. If multi-database listing is ever added, it
// MUST skip these names.
func introspectMySQL(ctx context.Context, db *sql.DB) ([]model.Database, error) {
	var databaseName sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&databaseName); err != nil {
		return nil, fmt.Errorf("read current database: %w", err)
	}

	builder := newMySQLBuilder()
	if databaseName.Valid && databaseName.String != "" {
		if err := queryMySQLTables(ctx, db, databaseName.String, builder); err != nil {
			return nil, err
		}
		if err := queryMySQLColumns(ctx, db, databaseName.String, builder); err != nil {
			return nil, err
		}
		if err := queryMySQLPrimaryKeys(ctx, db, databaseName.String, builder); err != nil {
			return nil, err
		}
		if err := queryMySQLForeignKeys(ctx, db, databaseName.String, builder); err != nil {
			return nil, err
		}
		if err := queryMySQLIndexes(ctx, db, databaseName.String, builder); err != nil {
			return nil, err
		}
	}

	return []model.Database{{
		Name:   databaseName.String,
		Tables: builder.tables(),
	}}, nil
}

type mysqlBuilder struct {
	order  []string
	byName map[string]*model.Table
}

func newMySQLBuilder() *mysqlBuilder {
	return &mysqlBuilder{byName: map[string]*model.Table{}}
}

func (b *mysqlBuilder) ensure(name, tableType string) *model.Table {
	if existing, ok := b.byName[name]; ok {
		return existing
	}
	table := &model.Table{Name: name, Type: tableType, Columns: []model.Column{}}
	b.byName[name] = table
	b.order = append(b.order, name)
	return table
}

func (b *mysqlBuilder) table(name string) *model.Table { return b.byName[name] }

func (b *mysqlBuilder) tables() []model.Table {
	out := make([]model.Table, 0, len(b.order))
	for _, name := range b.order {
		out = append(out, *b.byName[name])
	}
	return out
}

func queryMySQLTables(ctx context.Context, db *sql.DB, database string, builder *mysqlBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, TABLE_TYPE
		 FROM information_schema.TABLES
		 WHERE TABLE_SCHEMA = ? AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')
		 ORDER BY TABLE_NAME`, database)
	if err != nil {
		return fmt.Errorf("list mysql tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, tableType string
		if err := rows.Scan(&name, &tableType); err != nil {
			return fmt.Errorf("scan mysql table: %w", err)
		}
		builder.ensure(name, tableType)
	}
	return rows.Err()
}

func queryMySQLColumns(ctx context.Context, db *sql.DB, database string, builder *mysqlBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, ORDINAL_POSITION
		 FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = ?
		 ORDER BY TABLE_NAME, ORDINAL_POSITION`, database)
	if err != nil {
		return fmt.Errorf("list mysql columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			tableName, columnName, columnType, isNullable string
			columnDefault                                 sql.NullString
			position                                      int
		)
		if err := rows.Scan(&tableName, &columnName, &columnType, &isNullable, &columnDefault, &position); err != nil {
			return fmt.Errorf("scan mysql column: %w", err)
		}
		table := builder.table(tableName)
		if table == nil {
			continue
		}
		table.Columns = append(table.Columns, model.Column{
			Name:            columnName,
			DataType:        columnType,
			Nullable:        isNullable == "YES",
			Default:         mysqlStringPtr(columnDefault),
			OrdinalPosition: position,
		})
	}
	return rows.Err()
}

func queryMySQLPrimaryKeys(ctx context.Context, db *sql.DB, database string, builder *mysqlBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, COLUMN_NAME
		 FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA = ? AND INDEX_NAME = 'PRIMARY'
		 ORDER BY TABLE_NAME, SEQ_IN_INDEX`, database)
	if err != nil {
		return fmt.Errorf("list mysql primary keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			return fmt.Errorf("scan mysql primary key: %w", err)
		}
		table := builder.table(tableName)
		if table == nil {
			continue
		}
		if table.PrimaryKey == nil {
			table.PrimaryKey = &model.PrimaryKey{Name: "PRIMARY", Columns: []string{}}
		}
		table.PrimaryKey.Columns = append(table.PrimaryKey.Columns, columnName)
	}
	return rows.Err()
}

func queryMySQLForeignKeys(ctx context.Context, db *sql.DB, database string, builder *mysqlBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, CONSTRAINT_NAME, COLUMN_NAME,
		        REFERENCED_TABLE_SCHEMA, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
		 FROM information_schema.KEY_COLUMN_USAGE
		 WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL
		 ORDER BY TABLE_NAME, CONSTRAINT_NAME, ORDINAL_POSITION`, database)
	if err != nil {
		return fmt.Errorf("list mysql foreign keys: %w", err)
	}
	defer rows.Close()

	var (
		current           *model.ForeignKey
		currentTable      string
		currentConstraint string
	)
	for rows.Next() {
		var tableName, constraint, column, refSchema, refTable, refColumn string
		if err := rows.Scan(&tableName, &constraint, &column, &refSchema, &refTable, &refColumn); err != nil {
			return fmt.Errorf("scan mysql foreign key: %w", err)
		}
		table := builder.table(tableName)
		if table == nil {
			continue
		}
		if current == nil || tableName != currentTable || constraint != currentConstraint {
			table.ForeignKeys = append(table.ForeignKeys, model.ForeignKey{
				Name:              constraint,
				Columns:           []string{},
				ReferencedSchema:  refSchema,
				ReferencedTable:   refTable,
				ReferencedColumns: []string{},
			})
			current = &table.ForeignKeys[len(table.ForeignKeys)-1]
			currentTable = tableName
			currentConstraint = constraint
		}
		current.Columns = append(current.Columns, column)
		current.ReferencedColumns = append(current.ReferencedColumns, refColumn)
	}
	return rows.Err()
}

func queryMySQLIndexes(ctx context.Context, db *sql.DB, database string, builder *mysqlBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, COLUMN_NAME
		 FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA = ?
		 ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`, database)
	if err != nil {
		return fmt.Errorf("list mysql indexes: %w", err)
	}
	defer rows.Close()

	var (
		current      *model.Index
		currentTable string
		currentIndex string
	)
	for rows.Next() {
		var (
			tableName, indexName string
			nonUnique            int
			columnName           sql.NullString
		)
		if err := rows.Scan(&tableName, &indexName, &nonUnique, &columnName); err != nil {
			return fmt.Errorf("scan mysql index: %w", err)
		}
		table := builder.table(tableName)
		if table == nil {
			continue
		}
		if current == nil || tableName != currentTable || indexName != currentIndex {
			table.Indexes = append(table.Indexes, model.Index{
				Name:    indexName,
				Unique:  nonUnique == 0,
				Primary: indexName == "PRIMARY",
				Columns: []string{},
			})
			current = &table.Indexes[len(table.Indexes)-1]
			currentTable = tableName
			currentIndex = indexName
		}
		name := ""
		if columnName.Valid {
			name = columnName.String
		}
		current.Columns = append(current.Columns, name)
	}
	return rows.Err()
}

func mysqlStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	s := value.String
	return &s
}
