package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// introspectSQLite reads the connected database file using PRAGMA metadata.
// SQLite has no schema level; internal objects (sqlite_*) are excluded, so
// tables hang directly off model.Database.
func introspectSQLite(ctx context.Context, db *sql.DB) ([]model.Database, error) {
	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT name FROM pragma_database_list LIMIT 1`).Scan(&databaseName); err != nil {
		return nil, fmt.Errorf("read sqlite database name: %w", err)
	}

	builder := newSQLiteBuilder()
	if err := querySQLiteTables(ctx, db, builder); err != nil {
		return nil, err
	}
	for _, name := range builder.order {
		table := builder.tables[name]
		if err := querySQLiteColumns(ctx, db, table); err != nil {
			return nil, err
		}
		if err := querySQLiteIndexes(ctx, db, table); err != nil {
			return nil, err
		}
		if err := querySQLiteForeignKeys(ctx, db, table); err != nil {
			return nil, err
		}
	}

	return []model.Database{{Name: databaseName, Tables: builder.out()}}, nil
}

type sqliteBuilder struct {
	order  []string
	tables map[string]*model.Table
}

func newSQLiteBuilder() *sqliteBuilder {
	return &sqliteBuilder{tables: map[string]*model.Table{}}
}

func (b *sqliteBuilder) ensure(name, tableType string) *model.Table {
	if existing, ok := b.tables[name]; ok {
		return existing
	}
	table := &model.Table{Name: name, Type: tableType, Columns: []model.Column{}}
	b.tables[name] = table
	b.order = append(b.order, name)
	return table
}

func (b *sqliteBuilder) out() []model.Table {
	tables := make([]model.Table, 0, len(b.order))
	for _, name := range b.order {
		table := *b.tables[name]
		// Indexes/columns were appended in order; keep PK ordering by pk rank.
		tables = append(tables, table)
	}
	return tables
}

func querySQLiteTables(ctx context.Context, db *sql.DB, builder *sqliteBuilder) error {
	rows, err := db.QueryContext(ctx,
		`SELECT name, type FROM sqlite_master
		 WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		 ORDER BY name`)
	if err != nil {
		return fmt.Errorf("list sqlite tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, objectType string
		if err := rows.Scan(&name, &objectType); err != nil {
			return fmt.Errorf("scan sqlite table: %w", err)
		}
		tableType := "BASE TABLE"
		if objectType == "view" {
			tableType = "VIEW"
		}
		builder.ensure(name, tableType)
	}
	return rows.Err()
}

func querySQLiteColumns(ctx context.Context, db *sql.DB, table *model.Table) error {
	rows, err := db.QueryContext(ctx,
		`SELECT cid, name, type, "notnull", dflt_value, pk, hidden
		 FROM pragma_table_xinfo(?) ORDER BY cid`, table.Name)
	if err != nil {
		return fmt.Errorf("list sqlite columns for %q: %w", table.Name, err)
	}
	defer rows.Close()

	type pkEntry struct {
		column string
		rank   int
	}
	var pkEntries []pkEntry
	columnTypes := map[string]string{}

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultV   sql.NullString
			pkRank     int
			hidden     int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultV, &pkRank, &hidden); err != nil {
			return fmt.Errorf("scan sqlite column: %w", err)
		}
		columnTypes[name] = columnType
		table.Columns = append(table.Columns, model.Column{
			Name:            name,
			DataType:        columnType,
			Nullable:        notNull == 0,
			Default:         sqliteStringPtr(defaultV),
			OrdinalPosition: cid + 1,
			// hidden 2/3 mark VIRTUAL/STORED generated columns.
			Generated: hidden == 2 || hidden == 3,
		})
		if pkRank > 0 {
			pkEntries = append(pkEntries, pkEntry{column: name, rank: pkRank})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(pkEntries) > 0 {
		// Order by the composite PK rank.
		for i := 1; i < len(pkEntries); i++ {
			for j := i; j > 0 && pkEntries[j-1].rank > pkEntries[j].rank; j-- {
				pkEntries[j-1], pkEntries[j] = pkEntries[j], pkEntries[j-1]
			}
		}
		columns := make([]string, 0, len(pkEntries))
		for _, entry := range pkEntries {
			columns = append(columns, entry.column)
		}
		table.PrimaryKey = &model.PrimaryKey{Name: "PRIMARY", Columns: columns}
		// A single INTEGER PRIMARY KEY is SQLite's rowid alias (auto-assigned).
		if len(columns) == 1 && strings.Contains(strings.ToUpper(columnTypes[columns[0]]), "INT") {
			for i := range table.Columns {
				if table.Columns[i].Name == columns[0] {
					table.Columns[i].Identity = true
				}
			}
		}
	}
	return nil
}

func querySQLiteIndexes(ctx context.Context, db *sql.DB, table *model.Table) error {
	rows, err := db.QueryContext(ctx,
		`SELECT name, "unique", origin FROM pragma_index_list(?) ORDER BY name`, table.Name)
	if err != nil {
		return fmt.Errorf("list sqlite indexes for %q: %w", table.Name, err)
	}
	defer rows.Close()

	type indexMeta struct {
		name    string
		source  string
		unique  bool
		primary bool
	}
	var indexes []indexMeta
	for rows.Next() {
		var name, origin string
		var unique int
		if err := rows.Scan(&name, &unique, &origin); err != nil {
			return fmt.Errorf("scan sqlite index: %w", err)
		}
		// Represent the primary key index as "PRIMARY" and skip SQLite's noisy
		// auto-generated indexes for unique constraints. (A rowid INTEGER
		// PRIMARY KEY has no index at all.)
		displayName := name
		if origin == "pk" {
			displayName = "PRIMARY"
		} else if strings.HasPrefix(name, "sqlite_autoindex_") {
			continue
		}
		indexes = append(indexes, indexMeta{name: displayName, source: name, unique: unique == 1, primary: origin == "pk"})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, meta := range indexes {
		columnRows, err := db.QueryContext(ctx,
			`SELECT name FROM pragma_index_info(?) ORDER BY seqno`, meta.source)
		if err != nil {
			return fmt.Errorf("list sqlite index columns for %q: %w", meta.name, err)
		}
		columns := make([]string, 0)
		for columnRows.Next() {
			var columnName sql.NullString
			if err := columnRows.Scan(&columnName); err != nil {
				columnRows.Close()
				return fmt.Errorf("scan sqlite index column: %w", err)
			}
			name := ""
			if columnName.Valid {
				name = columnName.String
			}
			columns = append(columns, name)
		}
		err = columnRows.Err()
		columnRows.Close()
		if err != nil {
			return err
		}
		table.Indexes = append(table.Indexes, model.Index{
			Name:    meta.name,
			Unique:  meta.unique,
			Primary: meta.primary,
			Columns: columns,
		})
	}
	return nil
}

func querySQLiteForeignKeys(ctx context.Context, db *sql.DB, table *model.Table) error {
	rows, err := db.QueryContext(ctx,
		`SELECT id, seq, "table", "from", "to" FROM pragma_foreign_key_list(?) ORDER BY id, seq`, table.Name)
	if err != nil {
		return fmt.Errorf("list sqlite foreign keys for %q: %w", table.Name, err)
	}
	defer rows.Close()

	var (
		current   *model.ForeignKey
		currentID int
	)
	for rows.Next() {
		var (
			id       int
			seq      int
			refTable string
			fromCol  string
			toCol    sql.NullString
		)
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol); err != nil {
			return fmt.Errorf("scan sqlite foreign key: %w", err)
		}
		if current == nil || id != currentID {
			table.ForeignKeys = append(table.ForeignKeys, model.ForeignKey{
				Name:              fmt.Sprintf("fk_%s_%d", table.Name, id),
				Columns:           []string{},
				ReferencedSchema:  "",
				ReferencedTable:   refTable,
				ReferencedColumns: []string{},
			})
			current = &table.ForeignKeys[len(table.ForeignKeys)-1]
			currentID = id
		}
		current.Columns = append(current.Columns, fromCol)
		target := ""
		if toCol.Valid {
			target = toCol.String
		}
		current.ReferencedColumns = append(current.ReferencedColumns, target)
	}
	return rows.Err()
}

func sqliteStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	s := value.String
	return &s
}
