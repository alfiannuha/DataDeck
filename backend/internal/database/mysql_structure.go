package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// TableStructure implements StructureReader for MySQL. Introspection is scoped
// to the currently connected database (MySQL has no schema level below the
// database, so `schema` is ignored), with placeholders for every value —
// identifiers are never concatenated. Returns ErrTableNotFound when the table
// does not exist.
func (MySQL) TableStructure(ctx context.Context, db *sql.DB, schema, table string) (model.TableStructure, error) {
	objectType, exists, err := queryMySQLRelationExists(ctx, db, table)
	if err != nil {
		return model.TableStructure{}, err
	}
	if !exists {
		return model.TableStructure{}, fmt.Errorf("%w: %s", ErrTableNotFound, qualifiedLabel("", table))
	}

	columns, err := queryMySQLStructureColumns(ctx, db, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	primaryKey, pkPositions, err := queryMySQLStructurePrimaryKey(ctx, db, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	indexes, err := queryMySQLStructureIndexes(ctx, db, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	foreignKeys, err := queryMySQLStructureForeignKeys(ctx, db, table)
	if err != nil {
		return model.TableStructure{}, err
	}

	constraints, err := queryMySQLStructureConstraints(ctx, db, table, primaryKey, foreignKeys)
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

func queryMySQLRelationExists(ctx context.Context, db *sql.DB, table string) (string, bool, error) {
	var tableType string
	err := db.QueryRowContext(ctx,
		`SELECT TABLE_TYPE FROM information_schema.TABLES
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')`,
		table).Scan(&tableType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolve mysql relation: %w", err)
	}
	return tableType, true, nil
}

func queryMySQLStructureColumns(ctx context.Context, db *sql.DB, table string) ([]model.StructureColumn, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT COLUMN_NAME, COLUMN_TYPE, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, ORDINAL_POSITION,
		        EXTRA, CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE,
		        COLLATION_NAME, COLUMN_COMMENT, GENERATION_EXPRESSION
		 FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		 ORDER BY ORDINAL_POSITION`, table)
	if err != nil {
		return nil, fmt.Errorf("list mysql structure columns: %w", err)
	}
	defer rows.Close()

	columns := make([]model.StructureColumn, 0)
	for rows.Next() {
		var (
			columnName, columnType, dataType, isNullable, extra string
			columnDefault                                       []byte
			position                                            int
			maxLength, numericPrecision, numericScale           sql.NullInt64
			collation, comment, generationExpr                  sql.NullString
		)
		if err := rows.Scan(&columnName, &columnType, &dataType, &isNullable, &columnDefault, &position, &extra,
			&maxLength, &numericPrecision, &numericScale, &collation, &comment, &generationExpr); err != nil {
			return nil, fmt.Errorf("scan mysql structure column: %w", err)
		}
		columns = append(columns, model.StructureColumn{
			Name:            columnName,
			OrdinalPosition: position,
			DatabaseType:    columnType,
			NormalizedType:  normalizedType(dataType),
			Nullable:        isNullable == "YES",
			Default:         mysqlBytesPtr(columnDefault),
			Generated:       strings.Contains(strings.ToUpper(extra), "GENERATED"),
			Identity:        strings.Contains(strings.ToLower(extra), "auto_increment"),
			Length:          int(maxLength.Int64),
			Precision:       int(numericPrecision.Int64),
			Scale:           int(numericScale.Int64),
			Collation:       collation.String,
			Comment:         comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql structure columns: %w", err)
	}
	return columns, nil
}

func queryMySQLStructurePrimaryKey(ctx context.Context, db *sql.DB, table string) (*model.PrimaryKey, map[string]int, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT COLUMN_NAME FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY'
		 ORDER BY SEQ_IN_INDEX`, table)
	if err != nil {
		return nil, nil, fmt.Errorf("list mysql structure primary key: %w", err)
	}
	defer rows.Close()

	positions := make(map[string]int)
	var key *model.PrimaryKey
	idx := 0
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, nil, fmt.Errorf("scan mysql structure primary key: %w", err)
		}
		idx++
		positions[column] = idx
		if key == nil {
			key = &model.PrimaryKey{Name: "PRIMARY", Columns: []string{}}
		}
		key.Columns = append(key.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate mysql structure primary key: %w", err)
	}
	return key, positions, nil
}

func queryMySQLStructureIndexes(ctx context.Context, db *sql.DB, table string) ([]model.StructureIndex, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT INDEX_NAME, NON_UNIQUE, INDEX_TYPE, COLUMN_NAME
		 FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		 ORDER BY INDEX_NAME, SEQ_IN_INDEX`, table)
	if err != nil {
		return nil, fmt.Errorf("list mysql structure indexes: %w", err)
	}
	defer rows.Close()

	var (
		current      *model.StructureIndex
		currentIndex string
	)
	indexes := make([]model.StructureIndex, 0)
	for rows.Next() {
		var (
			indexName, indexType string
			nonUnique            int
			columnName           sql.NullString
		)
		if err := rows.Scan(&indexName, &nonUnique, &indexType, &columnName); err != nil {
			return nil, fmt.Errorf("scan mysql structure index: %w", err)
		}
		if current == nil || indexName != currentIndex {
			indexes = append(indexes, model.StructureIndex{
				Name:    indexName,
				Unique:  nonUnique == 0,
				Primary: indexName == "PRIMARY",
				Method:  indexType,
				Columns: []string{},
			})
			current = &indexes[len(indexes)-1]
			currentIndex = indexName
		}
		name := ""
		if columnName.Valid {
			name = columnName.String
		}
		current.Columns = append(current.Columns, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql structure indexes: %w", err)
	}
	return indexes, nil
}

func queryMySQLStructureConstraints(ctx context.Context, db *sql.DB, table string, primaryKey *model.PrimaryKey, foreignKeys []model.StructureForeignKey) ([]model.StructureConstraint, error) {
	uniqueColumns, uniqueNames, err := queryMySQLUniqueConstraints(ctx, db, table)
	if err != nil {
		return nil, err
	}
	constraints := make([]model.StructureConstraint, 0, len(uniqueNames))
	for _, name := range uniqueNames {
		constraints = append(constraints, model.StructureConstraint{
			Name: name, Type: "UNIQUE", Columns: uniqueColumns[name],
			Definition: "UNIQUE (" + strings.Join(uniqueColumns[name], ", ") + ")",
		})
	}

	checkConstraints, err := queryMySQLCheckConstraints(ctx, db, table)
	if err != nil {
		return nil, err
	}
	constraints = append(constraints, checkConstraints...)

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

func queryMySQLUniqueConstraints(ctx context.Context, db *sql.DB, table string) (map[string][]string, []string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT tc.CONSTRAINT_NAME, kcu.COLUMN_NAME
		 FROM information_schema.TABLE_CONSTRAINTS tc
		 JOIN information_schema.KEY_COLUMN_USAGE kcu
		   ON kcu.CONSTRAINT_SCHEMA = tc.CONSTRAINT_SCHEMA
		  AND kcu.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
		  AND kcu.TABLE_SCHEMA = tc.TABLE_SCHEMA
		  AND kcu.TABLE_NAME = tc.TABLE_NAME
		 WHERE tc.TABLE_SCHEMA = DATABASE() AND tc.TABLE_NAME = ? AND tc.CONSTRAINT_TYPE = 'UNIQUE'
		 ORDER BY tc.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`, table)
	if err != nil {
		return nil, nil, fmt.Errorf("list mysql unique constraints: %w", err)
	}
	defer rows.Close()

	columns := make(map[string][]string)
	order := make([]string, 0)
	for rows.Next() {
		var name, column string
		if err := rows.Scan(&name, &column); err != nil {
			return nil, nil, fmt.Errorf("scan mysql unique constraint: %w", err)
		}
		if _, seen := columns[name]; !seen {
			order = append(order, name)
		}
		columns[name] = append(columns[name], column)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate mysql unique constraints: %w", err)
	}
	return columns, order, nil
}

// queryMySQLCheckConstraints reads CHECK_CONSTRAINTS when the server supports
// it (MySQL 8.0.16+, MariaDB 10.2+). On older servers the section is simply
// absent rather than fabricated.
func queryMySQLCheckConstraints(ctx context.Context, db *sql.DB, table string) ([]model.StructureConstraint, error) {
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return nil, fmt.Errorf("read mysql version: %w", err)
	}
	if !mysqlSupportsCheckConstraints(version) {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx,
		`SELECT tc.CONSTRAINT_NAME, cc.CHECK_CLAUSE
		 FROM information_schema.TABLE_CONSTRAINTS tc
		 JOIN information_schema.CHECK_CONSTRAINTS cc
		   ON cc.CONSTRAINT_SCHEMA = tc.CONSTRAINT_SCHEMA
		  AND cc.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
		 WHERE tc.TABLE_SCHEMA = DATABASE() AND tc.TABLE_NAME = ? AND tc.CONSTRAINT_TYPE = 'CHECK'
		 ORDER BY tc.CONSTRAINT_NAME`, table)
	if err != nil {
		return nil, fmt.Errorf("list mysql check constraints: %w", err)
	}
	defer rows.Close()

	constraints := make([]model.StructureConstraint, 0)
	for rows.Next() {
		var name, clause string
		if err := rows.Scan(&name, &clause); err != nil {
			return nil, fmt.Errorf("scan mysql check constraint: %w", err)
		}
		constraints = append(constraints, model.StructureConstraint{
			Name: name, Type: "CHECK", Definition: "CHECK (" + clause + ")",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql check constraints: %w", err)
	}
	return constraints, nil
}

func queryMySQLStructureForeignKeys(ctx context.Context, db *sql.DB, table string) ([]model.StructureForeignKey, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT kcu.CONSTRAINT_NAME, kcu.COLUMN_NAME, kcu.REFERENCED_TABLE_SCHEMA, kcu.REFERENCED_TABLE_NAME,
		        kcu.REFERENCED_COLUMN_NAME, rc.UPDATE_RULE, rc.DELETE_RULE
		 FROM information_schema.KEY_COLUMN_USAGE kcu
		 LEFT JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
		   ON rc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA
		  AND rc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		 WHERE kcu.TABLE_SCHEMA = DATABASE() AND kcu.TABLE_NAME = ? AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		 ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`, table)
	if err != nil {
		return nil, fmt.Errorf("list mysql structure foreign keys: %w", err)
	}
	defer rows.Close()

	var (
		current     *model.StructureForeignKey
		currentName string
	)
	foreignKeys := make([]model.StructureForeignKey, 0)
	for rows.Next() {
		var (
			name, column, refSchema, refTable, refColumn, updateRule, deleteRule string
		)
		if err := rows.Scan(&name, &column, &refSchema, &refTable, &refColumn, &updateRule, &deleteRule); err != nil {
			return nil, fmt.Errorf("scan mysql structure foreign key: %w", err)
		}
		if current == nil || name != currentName {
			foreignKeys = append(foreignKeys, model.StructureForeignKey{
				Name:              name,
				Columns:           []string{},
				ReferencedSchema:  refSchema,
				ReferencedTable:   refTable,
				ReferencedColumns: []string{},
				OnUpdate:          updateRule,
				OnDelete:          deleteRule,
			})
			current = &foreignKeys[len(foreignKeys)-1]
			currentName = name
		}
		current.Columns = append(current.Columns, column)
		current.ReferencedColumns = append(current.ReferencedColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql structure foreign keys: %w", err)
	}
	return foreignKeys, nil
}

func mysqlBytesPtr(value []byte) *string {
	if value == nil {
		return nil
	}
	s := string(value)
	return &s
}

// mysqlSupportsCheckConstraints reports whether CHECK_CONSTRAINTS exists on the
// server. MySQL added enforced CHECK in 8.0.16, MariaDB in 10.2. It is never a
// blocker: unsupported versions simply have no CHECK constraint section.
func mysqlSupportsCheckConstraints(version string) bool {
	v := strings.ToLower(strings.TrimSpace(version))
	if strings.HasPrefix(v, "5.5.5-") {
		v = strings.TrimPrefix(v, "5.5.5-")
	}
	nums := leadingInts(v)
	if len(nums) < 2 {
		return true // unknown version → assume modern rather than hide known data
	}
	if strings.Contains(v, "mariadb") {
		if nums[0] > 10 || (nums[0] == 10 && len(nums) > 1 && nums[1] >= 2) {
			return true
		}
		return nums[0] > 10
	}
	if nums[0] > 8 {
		return true
	}
	if nums[0] < 8 || (nums[0] == 8 && len(nums) < 2) {
		return false
	}
	if nums[1] > 0 {
		return true
	}
	return len(nums) > 2 && nums[2] >= 16
}

func leadingInts(s string) []int {
	var out []int
	for _, part := range strings.Split(s, ".") {
		digitEnd := 0
		for digitEnd < len(part) && part[digitEnd] >= '0' && part[digitEnd] <= '9' {
			digitEnd++
		}
		if digitEnd == 0 {
			break
		}
		n, err := strconv.Atoi(part[:digitEnd])
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
