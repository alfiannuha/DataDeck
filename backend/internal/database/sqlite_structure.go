package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// TableStructure implements StructureReader for SQLite. Introspection is scoped
// to one relation in the connected database file via PRAGMA. SQLite has no
// schema level, so `schema` is ignored. Returns ErrTableNotFound when the
// table does not exist.
//
// SQLite exposes no CHECK constraints through its PRAGMA surface (they are
// embedded in the CREATE TABLE text), so the CHECK constraint section is
// intentionally absent rather than fabricated.
func (SQLite) TableStructure(ctx context.Context, db *sql.DB, schema, table string) (model.TableStructure, error) {
	objectType, exists, err := querySQLiteRelationExists(ctx, db, table)
	if err != nil {
		return model.TableStructure{}, err
	}
	if !exists {
		return model.TableStructure{}, fmt.Errorf("%w: %s", ErrTableNotFound, qualifiedLabel("", table))
	}

	columns, pkPositions, primaryKey, err := querySQLiteStructureColumns(ctx, db, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	indexes, err := querySQLiteStructureIndexes(ctx, db, table, primaryKey)
	if err != nil {
		return model.TableStructure{}, err
	}

	foreignKeys, err := querySQLiteStructureForeignKeys(ctx, db, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	constraints, err := querySQLiteStructureConstraints(ctx, db, table, primaryKey, foreignKeys)
	if err != nil {
		return model.TableStructure{}, err
	}

	for i := range columns {
		pos := pkPositions[columns[i].Name]
		columns[i].PrimaryKey = pos > 0
		columns[i].PrimaryKeyPosition = pos
		columns[i].Insertable, columns[i].Updatable = structureColumnMutability(columns[i].Generated, columns[i].Identity, columns[i].PrimaryKey)
	}
	structureColumnMutabilityForTable(objectType, columns)

	return model.TableStructure{
		ObjectType:  objectType,
		Columns:     columns,
		PrimaryKey:  primaryKey,
		Indexes:     indexes,
		Constraints: constraints,
		ForeignKeys: foreignKeys,
	}, nil
}

func querySQLiteRelationExists(ctx context.Context, db *sql.DB, table string) (string, bool, error) {
	var objectType string
	err := db.QueryRowContext(ctx,
		`SELECT type FROM sqlite_master
		 WHERE type IN ('table', 'view') AND name = ? AND name NOT LIKE 'sqlite_%'`,
		table).Scan(&objectType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolve sqlite relation: %w", err)
	}
	if objectType == "view" {
		return "VIEW", true, nil
	}
	return "BASE TABLE", true, nil
}

// querySQLiteStructureColumns reads columns in ordinal order via
// pragma_table_xinfo. pk rank is 1-based position in the declared primary key;
// a single INTEGER PRIMARY KEY is SQLite's rowid alias and is Identity, exactly
// as in the tree and mutation code.
func querySQLiteStructureColumns(ctx context.Context, db *sql.DB, table string) ([]model.StructureColumn, map[string]int, *model.PrimaryKey, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT cid, name, type, "notnull", dflt_value, pk, hidden
		 FROM pragma_table_xinfo(?) ORDER BY cid`, table)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list sqlite structure columns: %w", err)
	}
	defer rows.Close()

	columns := make([]model.StructureColumn, 0)
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
			return nil, nil, nil, fmt.Errorf("scan sqlite structure column: %w", err)
		}
		columnTypes[name] = columnType
		columns = append(columns, model.StructureColumn{
			Name:            name,
			OrdinalPosition: cid + 1,
			DatabaseType:    columnType,
			NormalizedType:  normalizedType(columnType),
			Nullable:        notNull == 0,
			Default:         sqliteStringPtr(defaultV),
			Generated:       hidden == 2 || hidden == 3,
		})
		if pkRank > 0 {
			pkEntries = append(pkEntries, pkEntry{column: name, rank: pkRank})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}

	positions := make(map[string]int)
	var primaryKey *model.PrimaryKey
	if len(pkEntries) > 0 {
		sort.Slice(pkEntries, func(i, j int) bool { return pkEntries[i].rank < pkEntries[j].rank })
		columnsList := make([]string, 0, len(pkEntries))
		for i, entry := range pkEntries {
			positions[entry.column] = i + 1
			columnsList = append(columnsList, entry.column)
		}
		primaryKey = &model.PrimaryKey{Name: "PRIMARY", Columns: columnsList}
		if len(columnsList) == 1 && strings.Contains(strings.ToUpper(columnTypes[columnsList[0]]), "INT") {
			for i := range columns {
				if columns[i].Name == columnsList[0] {
					columns[i].Identity = true
				}
			}
		}
	}
	return columns, positions, primaryKey, nil
}

// querySQLiteStructureIndexes lists indexes for the relation. All index_list
// rows are materialized before the per-index pragma_index_info queries so no
// query is ever issued while another result set holds the single SQLite
// connection. The PRIMARY index is derived from the declared primary key
// instead of pg-style auto indexes so dedicated sections stay consistent.
func querySQLiteStructureIndexes(ctx context.Context, db *sql.DB, table string, primaryKey *model.PrimaryKey) ([]model.StructureIndex, error) {
	type indexMeta struct {
		name    string
		source  string
		unique  bool
		primary bool
		partial int
	}
	var indexes []indexMeta

	rows, err := db.QueryContext(ctx,
		`SELECT name, "unique", origin, partial FROM pragma_index_list(?) ORDER BY name`, table)
	if err != nil {
		return nil, fmt.Errorf("list sqlite structure indexes: %w", err)
	}
	for rows.Next() {
		var name, origin string
		var unique, partial int
		if err := rows.Scan(&name, &unique, &origin, &partial); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan sqlite structure index: %w", err)
		}
		// Represent the primary key index as "PRIMARY"; skip SQLite's auto
		// indexes for unique constraints — those are surfaced as constraints.
		displayName := name
		if origin == "pk" {
			displayName = "PRIMARY"
		} else if strings.HasPrefix(name, "sqlite_autoindex_") {
			continue
		}
		indexes = append(indexes, indexMeta{name: displayName, source: name, unique: unique == 1, primary: origin == "pk", partial: partial})
	}
	iterErr := rows.Err()
	rows.Close()
	if iterErr != nil {
		return nil, iterErr
	}

	out := make([]model.StructureIndex, 0, len(indexes))
	for _, meta := range indexes {
		colNames, err := querySQLiteIndexColumnNames(ctx, db, meta.source)
		if err != nil {
			return nil, err
		}
		out = append(out, model.StructureIndex{
			Name:    meta.name,
			Unique:  meta.unique,
			Primary: meta.primary,
			Partial: meta.partial == 1,
			Columns: colNames,
		})
	}

	if primaryKey != nil {
		out = append(out, model.StructureIndex{
			Name:    primaryKey.Name,
			Unique:  true,
			Primary: true,
			Columns: primaryKey.Columns,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func querySQLiteStructureForeignKeys(ctx context.Context, db *sql.DB, table string) ([]model.StructureForeignKey, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, seq, "table", "from", "to", on_update, on_delete
		 FROM pragma_foreign_key_list(?) ORDER BY id, seq`, table)
	if err != nil {
		return nil, fmt.Errorf("list sqlite structure foreign keys: %w", err)
	}
	defer rows.Close()

	var (
		current   *model.StructureForeignKey
		currentID int
	)
	foreignKeys := make([]model.StructureForeignKey, 0)
	for rows.Next() {
		var (
			id                 int
			seq                int
			refTable           string
			fromCol            string
			toCol              sql.NullString
			onUpdate, onDelete sql.NullString
		)
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete); err != nil {
			return nil, fmt.Errorf("scan sqlite structure foreign key: %w", err)
		}
		if current == nil || id != currentID {
			foreignKeys = append(foreignKeys, model.StructureForeignKey{
				Name:              fmt.Sprintf("fk_%s_%d", table, id),
				Columns:           []string{},
				ReferencedTable:   refTable,
				ReferencedColumns: []string{},
				OnUpdate:          sqliteAction(onUpdate),
				OnDelete:          sqliteAction(onDelete),
			})
			current = &foreignKeys[len(foreignKeys)-1]
			currentID = id
		}
		current.Columns = append(current.Columns, fromCol)
		target := ""
		if toCol.Valid {
			target = toCol.String
		}
		current.ReferencedColumns = append(current.ReferencedColumns, target)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return foreignKeys, nil
}

func sqliteAction(value sql.NullString) string {
	if !value.Valid || value.String == "" {
		return "NO ACTION"
	}
	return strings.ToUpper(value.String)
}

// querySQLiteStructureConstraints exposes the constraints SQLite can report:
// the declared PRIMARY KEY, UNIQUE constraints (surfaced through their auto
// indexes) and foreign keys. CHECK constraints are not reliably exposed and are
// intentionally not invented. Unique index names are materialized before the
// per-index columns queries so no query runs while another result set holds
// the single SQLite connection.
func querySQLiteStructureConstraints(ctx context.Context, db *sql.DB, table string, primaryKey *model.PrimaryKey, foreignKeys []model.StructureForeignKey) ([]model.StructureConstraint, error) {
	uniqueIndexes := make([]string, 0)

	rows, err := db.QueryContext(ctx,
		`SELECT name, "unique" FROM pragma_index_list(?) WHERE origin = 'u' ORDER BY name`, table)
	if err != nil {
		return nil, fmt.Errorf("list sqlite unique constraints: %w", err)
	}
	for rows.Next() {
		var name string
		var unique int
		if err := rows.Scan(&name, &unique); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan sqlite unique constraint: %w", err)
		}
		uniqueIndexes = append(uniqueIndexes, name)
	}
	iterErr := rows.Err()
	rows.Close()
	if iterErr != nil {
		return nil, iterErr
	}

	// A rowid INTEGER PRIMARY KEY needs no index; on a WITHOUT ROWID / composite
	// primary key the pragma also reports origin='pk' (not 'u'), so "unique"
	// true-or-false is irrelevant here — 'u' indexes are unique by definition.
	constraints := make([]model.StructureConstraint, 0)
	for _, name := range uniqueIndexes {
		colNames, err := querySQLiteIndexColumnNames(ctx, db, name)
		if err != nil {
			return nil, err
		}
		constraints = append(constraints, model.StructureConstraint{
			Name: name, Type: "UNIQUE", Columns: colNames,
			Definition: "UNIQUE (" + strings.Join(colNames, ", ") + ")",
		})
	}

	if primaryKey != nil {
		constraints = append(constraints, model.StructureConstraint{
			Name: primaryKey.Name, Type: "PRIMARY KEY", Columns: primaryKey.Columns,
		})
	}
	for _, fk := range foreignKeys {
		constraints = append(constraints, model.StructureConstraint{
			Name: fk.Name, Type: "FOREIGN KEY", Columns: fk.Columns,
		})
	}
	sort.Slice(constraints, func(i, j int) bool { return constraints[i].Name < constraints[j].Name })
	return constraints, nil
}

func querySQLiteIndexColumnNames(ctx context.Context, db *sql.DB, index string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name FROM pragma_index_info(?) ORDER BY seqno`, index)
	if err != nil {
		return nil, fmt.Errorf("list sqlite index columns for %q: %w", index, err)
	}
	defer rows.Close()

	columns := make([]string, 0)
	for rows.Next() {
		var columnName sql.NullString
		if err := rows.Scan(&columnName); err != nil {
			return nil, fmt.Errorf("scan sqlite index column: %w", err)
		}
		name := ""
		if columnName.Valid {
			name = columnName.String
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}
