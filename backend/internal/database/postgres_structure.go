package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// TableStructure implements StructureReader for PostgreSQL. It reads metadata
// for exactly one relation (never the whole database) with bound parameters
// only; identifiers are never concatenated into the queries. Returns
// ErrTableNotFound when the relation does not exist.
func (Postgres) TableStructure(ctx context.Context, db *sql.DB, schema, table string) (model.TableStructure, error) {
	objectType, exists, err := queryRelationExists(ctx, db, schema, table)
	if err != nil {
		return model.TableStructure{}, err
	}
	if !exists {
		return model.TableStructure{}, fmt.Errorf("%w: %s", ErrTableNotFound, qualifiedLabel(schema, table))
	}

	columns, err := queryStructureColumns(ctx, db, schema, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	primaryKey, pkPositions, err := queryStructurePrimaryKey(ctx, db, schema, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	indexes, err := queryStructureIndexes(ctx, db, schema, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	constraints, err := queryStructureConstraints(ctx, db, schema, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	foreignKeys, err := queryStructureForeignKeys(ctx, db, schema, table)
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

// queryRelationExists resolves the relation kind for (schema, table). A
// missing relation reports exists=false (callers map that to ErrTableNotFound).
func queryRelationExists(ctx context.Context, db *sql.DB, schema, table string) (string, bool, error) {
	var relkind string
	err := db.QueryRowContext(ctx,
		`SELECT c.relkind
		 FROM pg_catalog.pg_class c
		 JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1 AND c.relname = $2 AND c.relkind IN ('r', 'p', 'v', 'm', 'f')`,
		schema, table).Scan(&relkind)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolve relation: %w", err)
	}
	return relationType(relkind), true, nil
}

// queryStructureColumns reads column metadata in ordinal order. Length,
// precision and scale come from information_schema (reliable for the standard
// types) and are matched by ordinal position.
func queryStructureColumns(ctx context.Context, db *sql.DB, schema, table string) ([]model.StructureColumn, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT a.attname,
		        pg_catalog.format_type(a.atttypid, a.atttypmod),
		        NOT a.attnotnull AS nullable,
		        pg_catalog.pg_get_expr(d.adbin, d.adrelid) AS default_expr,
		        a.attnum,
		        a.attgenerated <> '' AS generated,
		        a.attidentity <> '' AS identity,
		        collect.collname AS collation,
		        pg_catalog.col_description(c.oid, a.attnum) AS comment
		 FROM pg_catalog.pg_attribute a
		 JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		 LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		 LEFT JOIN pg_catalog.pg_collation collect ON collect.oid = a.attcollation
		 WHERE a.attnum > 0 AND NOT a.attisdropped
		   AND n.nspname = $1 AND c.relname = $2
		 ORDER BY a.attnum`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("list structure columns: %w", err)
	}
	defer rows.Close()

	columns := make([]model.StructureColumn, 0)
	for rows.Next() {
		var (
			column, dataType string
			nullable         bool
			defaultExpr      sql.NullString
			position         int
			generated        bool
			identity         bool
			collation        sql.NullString
			comment          sql.NullString
		)
		if err := rows.Scan(&column, &dataType, &nullable, &defaultExpr, &position, &generated, &identity, &collation, &comment); err != nil {
			return nil, fmt.Errorf("scan structure column: %w", err)
		}
		columns = append(columns, model.StructureColumn{
			Name:            column,
			OrdinalPosition: position,
			DatabaseType:    dataType,
			NormalizedType:  normalizedType(dataType),
			Nullable:        nullable,
			Default:         stringPtr(defaultExpr),
			Generated:       generated,
			Identity:        identity,
			Collation:       collation.String,
			Comment:         comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate structure columns: %w", err)
	}

	if err := augmentStructureColumnAttrs(ctx, db, schema, table, columns); err != nil {
		return nil, err
	}
	return columns, nil
}

// augmentStructureColumnAttrs fills length/precision/scale from
// information_schema, keyed by ordinal position.
func augmentStructureColumnAttrs(ctx context.Context, db *sql.DB, schema, table string, columns []model.StructureColumn) error {
	rows, err := db.QueryContext(ctx,
		`SELECT ordinal_position, character_maximum_length, numeric_precision, numeric_scale
		 FROM information_schema.columns
		 WHERE table_schema = $1 AND table_name = $2
		 ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return fmt.Errorf("list structure column attributes: %w", err)
	}
	defer rows.Close()

	byPosition := make(map[int]struct {
		length    int
		precision int
		scale     int
	})
	for rows.Next() {
		var position, length, precision, scale int
		var lengthVal, precisionVal, scaleVal sql.NullInt64
		if err := rows.Scan(&position, &lengthVal, &precisionVal, &scaleVal); err != nil {
			return fmt.Errorf("scan structure column attributes: %w", err)
		}
		length = int(lengthVal.Int64)
		precision = int(precisionVal.Int64)
		scale = int(scaleVal.Int64)
		byPosition[position] = struct {
			length    int
			precision int
			scale     int
		}{length, precision, scale}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate structure column attributes: %w", err)
	}

	for i := range columns {
		attrs, ok := byPosition[columns[i].OrdinalPosition]
		if !ok {
			continue
		}
		columns[i].Length = attrs.length
		columns[i].Precision = attrs.precision
		columns[i].Scale = attrs.scale
	}
	return nil
}

// queryStructurePrimaryKey returns the declared primary key (in constraint
// order) and each column's 1-based position within it.
func queryStructurePrimaryKey(ctx context.Context, db *sql.DB, schema, table string) (*model.PrimaryKey, map[string]int, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT con.conname, a.attname
		 FROM pg_catalog.pg_constraint con
		 JOIN pg_catalog.pg_class t ON t.oid = con.conrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		 JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord) ON true
		 JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
		 WHERE con.contype = 'p' AND n.nspname = $1 AND t.relname = $2
		 ORDER BY k.ord`, schema, table)
	if err != nil {
		return nil, nil, fmt.Errorf("list structure primary key: %w", err)
	}
	defer rows.Close()

	positions := make(map[string]int)
	var key *model.PrimaryKey
	idx := 0
	for rows.Next() {
		var name, column string
		if err := rows.Scan(&name, &column); err != nil {
			return nil, nil, fmt.Errorf("scan structure primary key: %w", err)
		}
		idx++
		positions[column] = idx
		if key == nil {
			key = &model.PrimaryKey{Name: name, Columns: []string{}}
		}
		key.Columns = append(key.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate structure primary key: %w", err)
	}
	return key, positions, nil
}

// queryStructureConstraints lists PRIMARY KEY, UNIQUE, CHECK and FOREIGN KEY
// constraints. The PRIMARY KEY entry mirrors the dedicated primary_key section
// (declared constraints only; NOT NULL is a column attribute, never invented as
// a table constraint). CHECK/UNIQUE carry their definition text; PK/FK leave it
// empty because dedicated sections already carry their columns and actions.
func queryStructureConstraints(ctx context.Context, db *sql.DB, schema, table string) ([]model.StructureConstraint, error) {
	colsByConstraint, err := queryConstraintColumns(ctx, db, schema, table)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx,
		`SELECT con.conname, con.contype, pg_catalog.pg_get_constraintdef(con.oid)
		 FROM pg_catalog.pg_constraint con
		 JOIN pg_catalog.pg_class t ON t.oid = con.conrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		 WHERE con.contype IN ('p', 'u', 'c', 'f') AND n.nspname = $1 AND t.relname = $2
		 ORDER BY con.conname`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("list structure constraints: %w", err)
	}
	defer rows.Close()

	constraints := make([]model.StructureConstraint, 0)
	for rows.Next() {
		var name, typ, definition string
		if err := rows.Scan(&name, &typ, &definition); err != nil {
			return nil, fmt.Errorf("scan structure constraint: %w", err)
		}
		constraint := model.StructureConstraint{
			Name:       name,
			Type:       structureConstraintType(typ),
			Columns:    colsByConstraint[name],
			Definition: definition,
		}
		if constraint.Type != "CHECK" && constraint.Type != "UNIQUE" {
			constraint.Definition = ""
		}
		constraints = append(constraints, constraint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate structure constraints: %w", err)
	}

	sort.Slice(constraints, func(i, j int) bool { return constraints[i].Name < constraints[j].Name })
	return constraints, nil
}

// queryConstraintColumns maps constraint name to its ordered column list.
func queryConstraintColumns(ctx context.Context, db *sql.DB, schema, table string) (map[string][]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT con.conname, a.attname
		 FROM pg_catalog.pg_constraint con
		 JOIN pg_catalog.pg_class t ON t.oid = con.conrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		 JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord) ON true
		 JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
		 WHERE con.contype IN ('p', 'u', 'f') AND n.nspname = $1 AND t.relname = $2
		 ORDER BY con.conname, k.ord`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("list structure constraint columns: %w", err)
	}
	defer rows.Close()

	grouped := make(map[string][]string)
	for rows.Next() {
		var name, column string
		if err := rows.Scan(&name, &column); err != nil {
			return nil, fmt.Errorf("scan structure constraint columns: %w", err)
		}
		grouped[name] = append(grouped[name], column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate structure constraint columns: %w", err)
	}
	return grouped, nil
}

// queryStructureIndexes lists indexes for the relation. The PRIMARY index is
// flagged from pg_index.indisprimary so it is listed exactly once and the
// dedicated primary_key section stays authoritative. Expression index parts use
// the existing empty-name convention.
func queryStructureIndexes(ctx context.Context, db *sql.DB, schema, table string) ([]model.StructureIndex, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT ic.relname, i.indisunique, i.indisprimary, am.amname, i.indpred IS NOT NULL AS partial, a.attname
		 FROM pg_catalog.pg_index i
		 JOIN pg_catalog.pg_class t ON t.oid = i.indrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		 JOIN pg_catalog.pg_class ic ON ic.oid = i.indexrelid
		 JOIN pg_catalog.pg_am am ON am.oid = ic.relam
		 JOIN LATERAL unnest(i.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord) ON true
		 LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
		 WHERE t.relkind IN ('r', 'p', 'm') AND n.nspname = $1 AND t.relname = $2
		 ORDER BY ic.relname, k.ord`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("list structure indexes: %w", err)
	}
	defer rows.Close()

	var (
		current      *model.StructureIndex
		currentIndex string
	)
	indexes := make([]model.StructureIndex, 0)
	for rows.Next() {
		var (
			index                       string
			unique, primary, partial    bool
			method                      string
			column                      sql.NullString
		)
		if err := rows.Scan(&index, &unique, &primary, &method, &partial, &column); err != nil {
			return nil, fmt.Errorf("scan structure index: %w", err)
		}
		if current == nil || currentIndex != index {
			indexes = append(indexes, model.StructureIndex{
				Name:    index,
				Unique:  unique,
				Primary: primary,
				Method:  method,
				Partial: partial,
				Columns: []string{},
			})
			current = &indexes[len(indexes)-1]
			currentIndex = index
		}
		name := ""
		if column.Valid {
			name = column.String
		}
		current.Columns = append(current.Columns, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate structure indexes: %w", err)
	}

	sort.Slice(indexes, func(i, j int) bool { return indexes[i].Name < indexes[j].Name })
	return indexes, nil
}

// queryStructureForeignKeys lists foreign keys with referential actions,
// preserving composite column ordering.
func queryStructureForeignKeys(ctx context.Context, db *sql.DB, schema, table string) ([]model.StructureForeignKey, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT con.conname, a.attname, fn.nspname, ft.relname, fa.attname,
		        con.confupdtype, con.confdeltype
		 FROM pg_catalog.pg_constraint con
		 JOIN pg_catalog.pg_class t ON t.oid = con.conrelid
		 JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		 JOIN pg_catalog.pg_class ft ON ft.oid = con.confrelid
		 JOIN pg_catalog.pg_namespace fn ON fn.oid = ft.relnamespace
		 JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord) ON true
		 JOIN LATERAL unnest(con.confkey) WITH ORDINALITY AS rk(attnum, ord) ON rk.ord = k.ord
		 JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
		 JOIN pg_catalog.pg_attribute fa ON fa.attrelid = con.confrelid AND fa.attnum = rk.attnum
		 WHERE con.contype = 'f' AND n.nspname = $1 AND t.relname = $2
		 ORDER BY con.conname, k.ord`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("list structure foreign keys: %w", err)
	}
	defer rows.Close()

	var (
		current     *model.StructureForeignKey
		currentName string
	)
	foreignKeys := make([]model.StructureForeignKey, 0)
	for rows.Next() {
		var (
			name, column, refSchema, refTable, refColumn string
			updateType, deleteType                       string
		)
		if err := rows.Scan(&name, &column, &refSchema, &refTable, &refColumn, &updateType, &deleteType); err != nil {
			return nil, fmt.Errorf("scan structure foreign key: %w", err)
		}
		if current == nil || currentName != name {
			foreignKeys = append(foreignKeys, model.StructureForeignKey{
				Name:              name,
				Columns:           []string{},
				ReferencedSchema:  refSchema,
				ReferencedTable:   refTable,
				ReferencedColumns: []string{},
				OnUpdate:          referentialAction(updateType),
				OnDelete:          referentialAction(deleteType),
			})
			current = &foreignKeys[len(foreignKeys)-1]
			currentName = name
		}
		current.Columns = append(current.Columns, column)
		current.ReferencedColumns = append(current.ReferencedColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate structure foreign keys: %w", err)
	}
	return foreignKeys, nil
}

// structureConstraintType maps pg_constraint.contype to the canonical label.
func structureConstraintType(contype string) string {
	switch contype {
	case "p":
		return "PRIMARY KEY"
	case "u":
		return "UNIQUE"
	case "c":
		return "CHECK"
	case "f":
		return "FOREIGN KEY"
	default:
		return contype
	}
}

// referentialAction maps pg_constraint.confupdtype/confdeltype to a label.
func referentialAction(action string) string {
	switch action {
	case "c":
		return "CASCADE"
	case "r":
		return "RESTRICT"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	case "a":
		return "NO ACTION"
	default:
		return action
	}
}
