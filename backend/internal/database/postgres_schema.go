package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// systemSchemaFilter excludes PostgreSQL's internal schemas from the explorer:
// the two information schemas and every schema whose name starts with "pg_"
// (pg_toast, pg_temp_*, …). The backslash escapes the underscore so the filter
// does not accidentally match unrelated names.
const systemSchemaFilter = `n.nspname NOT IN ('pg_catalog', 'information_schema')` +
	` AND n.nspname NOT LIKE 'pg\_%'`

// Introspect reads the current database's user-visible structure and returns it
// as a database-neutral tree. The connection targets a single database, so the
// result always contains exactly one model.Database.
func (Postgres) Introspect(ctx context.Context, db *sql.DB) ([]model.Database, error) {
	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		return nil, fmt.Errorf("read current database: %w", err)
	}

	schemaNames, err := querySchemaNames(ctx, db)
	if err != nil {
		return nil, err
	}

	builder := newSchemaBuilder()
	for _, name := range schemaNames {
		builder.ensureSchema(name)
	}
	if err := queryTables(ctx, db, builder); err != nil {
		return nil, err
	}
	if err := queryColumns(ctx, db, builder); err != nil {
		return nil, err
	}
	if err := queryPrimaryKeys(ctx, db, builder); err != nil {
		return nil, err
	}
	if err := queryForeignKeys(ctx, db, builder); err != nil {
		return nil, err
	}
	if err := queryIndexes(ctx, db, builder); err != nil {
		return nil, err
	}

	return []model.Database{{Name: databaseName, Schemas: builder.schemas()}}, nil
}

type tableKey struct{ schema, table string }

// schemaBuilder assembles the nested tree while keeping stable pointers to
// tables during construction.
type schemaBuilder struct {
	order        []string
	schemaSet    map[string]bool
	schemaTables map[string][]*model.Table
	tables       map[tableKey]*model.Table
}

func newSchemaBuilder() *schemaBuilder {
	return &schemaBuilder{
		schemaSet:    map[string]bool{},
		schemaTables: map[string][]*model.Table{},
		tables:       map[tableKey]*model.Table{},
	}
}

func (b *schemaBuilder) ensureSchema(name string) {
	if b.schemaSet[name] {
		return
	}
	b.schemaSet[name] = true
	b.order = append(b.order, name)
}

func (b *schemaBuilder) ensureTable(schema, table, tableType string) *model.Table {
	b.ensureSchema(schema)
	key := tableKey{schema: schema, table: table}
	if existing, ok := b.tables[key]; ok {
		return existing
	}
	t := &model.Table{Schema: schema, Name: table, Type: tableType, Columns: []model.Column{}}
	b.tables[key] = t
	b.schemaTables[schema] = append(b.schemaTables[schema], t)
	return t
}

func (b *schemaBuilder) table(schema, table string) *model.Table {
	return b.tables[tableKey{schema: schema, table: table}]
}

func (b *schemaBuilder) schemas() []model.Schema {
	out := make([]model.Schema, 0, len(b.order))
	for _, name := range b.order {
		pointerTables := b.schemaTables[name]
		tables := make([]model.Table, 0, len(pointerTables))
		for _, t := range pointerTables {
			tables = append(tables, *t)
		}
		out = append(out, model.Schema{Name: name, Tables: tables})
	}
	return out
}

func querySchemaNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT n.nspname FROM pg_catalog.pg_namespace n WHERE `+systemSchemaFilter+` ORDER BY n.nspname`)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan schema: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schemas: %w", err)
	}
	return names, nil
}

func queryTables(ctx context.Context, db *sql.DB, builder *schemaBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT n.nspname, c.relname, c.relkind
		 FROM pg_catalog.pg_class c
		 JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		 WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f') AND `+systemSchemaFilter+`
		 ORDER BY n.nspname, c.relname`)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table, relkind string
		if err := rows.Scan(&schema, &table, &relkind); err != nil {
			return fmt.Errorf("scan table: %w", err)
		}
		builder.ensureTable(schema, table, relationType(relkind))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tables: %w", err)
	}
	return nil
}

func queryColumns(ctx context.Context, db *sql.DB, builder *schemaBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT n.nspname, c.relname, a.attname,
		        pg_catalog.format_type(a.atttypid, a.atttypmod),
		        NOT a.attnotnull AS nullable,
		        pg_catalog.pg_get_expr(d.adbin, d.adrelid) AS default_expr,
		        a.attnum,
		        a.attgenerated <> '' AS generated,
		        a.attidentity <> '' AS identity
		 FROM pg_catalog.pg_attribute a
		 JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		 LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		 WHERE a.attnum > 0 AND NOT a.attisdropped
		   AND c.relkind IN ('r', 'p', 'v', 'm', 'f') AND `+systemSchemaFilter+`
		 ORDER BY n.nspname, c.relname, a.attnum`)
	if err != nil {
		return fmt.Errorf("list columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			schema, table, column, dataType string
			nullable                        bool
			defaultExpr                     sql.NullString
			position                        int
			generated, identity             bool
		)
		if err := rows.Scan(&schema, &table, &column, &dataType, &nullable, &defaultExpr, &position, &generated, &identity); err != nil {
			return fmt.Errorf("scan column: %w", err)
		}
		t := builder.table(schema, table)
		if t == nil {
			continue
		}
		t.Columns = append(t.Columns, model.Column{
			Name:            column,
			DataType:        dataType,
			Nullable:        nullable,
			Default:         stringPtr(defaultExpr),
			OrdinalPosition: position,
			Generated:       generated,
			Identity:        identity,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate columns: %w", err)
	}
	return nil
}

func queryPrimaryKeys(ctx context.Context, db *sql.DB, builder *schemaBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT n.nspname, t.relname, con.conname, a.attname
		 FROM pg_catalog.pg_constraint con
		 JOIN pg_catalog.pg_class t ON t.oid = con.conrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		 JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord) ON true
		 JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
		 WHERE con.contype = 'p' AND `+systemSchemaFilter+`
		 ORDER BY n.nspname, t.relname, k.ord`)
	if err != nil {
		return fmt.Errorf("list primary keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table, constraint, column string
		if err := rows.Scan(&schema, &table, &constraint, &column); err != nil {
			return fmt.Errorf("scan primary key: %w", err)
		}
		t := builder.table(schema, table)
		if t == nil {
			continue
		}
		if t.PrimaryKey == nil || t.PrimaryKey.Name != constraint {
			t.PrimaryKey = &model.PrimaryKey{Name: constraint, Columns: []string{}}
		}
		t.PrimaryKey.Columns = append(t.PrimaryKey.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate primary keys: %w", err)
	}
	return nil
}

func queryForeignKeys(ctx context.Context, db *sql.DB, builder *schemaBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT n.nspname, t.relname, con.conname, a.attname,
		        fn.nspname AS ref_schema, ft.relname AS ref_table, fa.attname AS ref_column
		 FROM pg_catalog.pg_constraint con
		 JOIN pg_catalog.pg_class t ON t.oid = con.conrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		 JOIN pg_catalog.pg_class ft ON ft.oid = con.confrelid
		 JOIN pg_catalog.pg_namespace fn ON fn.oid = ft.relnamespace
		 JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord) ON true
		 JOIN LATERAL unnest(con.confkey) WITH ORDINALITY AS rk(attnum, ord) ON rk.ord = k.ord
		 JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
		 JOIN pg_catalog.pg_attribute fa ON fa.attrelid = con.confrelid AND fa.attnum = rk.attnum
		 WHERE con.contype = 'f' AND `+systemSchemaFilter+`
		 ORDER BY n.nspname, t.relname, con.conname, k.ord`)
	if err != nil {
		return fmt.Errorf("list foreign keys: %w", err)
	}
	defer rows.Close()

	var (
		current           *model.ForeignKey
		currentKey        tableKey
		currentConstraint string
	)
	for rows.Next() {
		var schema, table, constraint, column, refSchema, refTable, refColumn string
		if err := rows.Scan(&schema, &table, &constraint, &column, &refSchema, &refTable, &refColumn); err != nil {
			return fmt.Errorf("scan foreign key: %w", err)
		}
		t := builder.table(schema, table)
		if t == nil {
			continue
		}
		key := tableKey{schema: schema, table: table}
		if current == nil || key != currentKey || constraint != currentConstraint {
			t.ForeignKeys = append(t.ForeignKeys, model.ForeignKey{
				Name:              constraint,
				Columns:           []string{},
				ReferencedSchema:  refSchema,
				ReferencedTable:   refTable,
				ReferencedColumns: []string{},
			})
			current = &t.ForeignKeys[len(t.ForeignKeys)-1]
			currentKey = key
			currentConstraint = constraint
		}
		current.Columns = append(current.Columns, column)
		current.ReferencedColumns = append(current.ReferencedColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate foreign keys: %w", err)
	}
	return nil
}

func queryIndexes(ctx context.Context, db *sql.DB, builder *schemaBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT n.nspname, t.relname, ic.relname, i.indisunique, i.indisprimary, a.attname
		 FROM pg_catalog.pg_index i
		 JOIN pg_catalog.pg_class t ON t.oid = i.indrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		 JOIN pg_catalog.pg_class ic ON ic.oid = i.indexrelid
		 JOIN LATERAL unnest(i.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord) ON true
		 LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
		 WHERE t.relkind IN ('r', 'p', 'm') AND `+systemSchemaFilter+`
		 ORDER BY n.nspname, t.relname, ic.relname, k.ord`)
	if err != nil {
		return fmt.Errorf("list indexes: %w", err)
	}
	defer rows.Close()

	var (
		current    *model.Index
		currentKey tableKey
	)
	for rows.Next() {
		var (
			schema, table, index string
			unique, primary      bool
			column               sql.NullString
		)
		if err := rows.Scan(&schema, &table, &index, &unique, &primary, &column); err != nil {
			return fmt.Errorf("scan index: %w", err)
		}
		t := builder.table(schema, table)
		if t == nil {
			continue
		}
		key := tableKey{schema: schema, table: table}
		if current == nil || key != currentKey || current.Name != index {
			t.Indexes = append(t.Indexes, model.Index{
				Name:    index,
				Unique:  unique,
				Primary: primary,
				Columns: []string{},
			})
			current = &t.Indexes[len(t.Indexes)-1]
			currentKey = key
		}
		name := ""
		if column.Valid {
			name = column.String
		}
		current.Columns = append(current.Columns, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate indexes: %w", err)
	}
	return nil
}

func relationType(relkind string) string {
	switch relkind {
	case "r":
		return "BASE TABLE"
	case "p":
		return "PARTITIONED TABLE"
	case "v":
		return "VIEW"
	case "m":
		return "MATERIALIZED VIEW"
	case "f":
		return "FOREIGN TABLE"
	default:
		return relkind
	}
}

func stringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}
