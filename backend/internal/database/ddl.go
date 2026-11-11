package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Structure change operations (PRF-04/Phase 2). One request applies exactly one
// operation; the manager builds the engine-specific statement(s) and executes
// them against the target database. Identifiers are always quoted with the
// driver's identifier quote; literal defaults are emitted as SQL literals and
// raw fragments (type/expression/where) are validated to keep each request a
// single, non-chained statement.
const (
	OpRenameTable    = "rename_table"
	OpAddColumn      = "add_column"
	OpDropColumn     = "drop_column"
	OpAlterColumn    = "alter_column"
	OpCreateIndex    = "create_index"
	OpDropIndex      = "drop_index"
	OpAddConstraint  = "add_constraint"
	OpDropConstraint = "drop_constraint"
)

// ErrInvalidStructureChange marks a schema-change request that failed client-side
// validation (missing/invalid fields, unsupported characters). Handlers map it
// to VALIDATION_ERROR.
var ErrInvalidStructureChange = errors.New("database: invalid structure change")

// ColumnDefinition is an engine-neutral column spec for add/alter.
type ColumnDefinition struct {
	Name         string
	Type         string
	Nullable     bool
	Default      *string
	IsExpression bool
}

// IndexColumn is one key part of an index.
type IndexColumn struct {
	Name      string
	Direction string // "", "asc", "desc"
}

// IndexDefinition describes an index to create.
type IndexDefinition struct {
	Name    string
	Unique  bool
	Method  string
	Columns []IndexColumn
	Where   string
}

// ReferenceDefinition is the target of a foreign key.
type ReferenceDefinition struct {
	Schema   string
	Table    string
	Columns  []string
	OnUpdate string
	OnDelete string
}

// ConstraintDefinition describes a table constraint to add.
type ConstraintDefinition struct {
	Name       string
	Type       string // PRIMARY KEY | UNIQUE | CHECK | FOREIGN KEY
	Columns    []string
	Expression string
	Reference  *ReferenceDefinition
}

// StructureChange is one requested schema edit. Only the fields for Operation
// are read; the manager validates the rest.
type StructureChange struct {
	Operation      string
	Schema         string
	Table          string
	NewName        string
	Column         *ColumnDefinition
	ColumnName     string
	NewColumn      *ColumnDefinition
	DefaultAction  string // alter_column: "", "set", "drop"
	Index          *IndexDefinition
	IndexName      string
	Constraint     *ConstraintDefinition
	ConstraintName string
	ConstraintType string
}

// changeError marks a client-side (validation) problem with a change request.
type changeError struct{ msg string }

func (e changeError) Error() string { return e.msg }

// Unwrap lets handlers detect a client validation failure with errors.Is.
func (e changeError) Unwrap() error { return ErrInvalidStructureChange }

func invalidChange(format string, args ...any) error {
	return changeError{msg: fmt.Sprintf(format, args...)}
}

// StructureCapabilitiesFor reports which schema edits the engine supports.
func StructureCapabilitiesFor(driver model.Driver) model.StructureCapabilities {
	switch driver {
	case model.DriverPostgres:
		return model.StructureCapabilities{
			RenameTable: true, AddColumn: true, DropColumn: true, AlterColumn: true,
			CreateIndex: true, DropIndex: true, AddConstraint: true, DropConstraint: true,
			PartialIndex: true, IndexMethod: true,
		}
	case model.DriverMySQL:
		// MySQL DDL implicitly commits, so a multi-statement change is not
		// atomic; the manager rejects multi-statement changes for MySQL.
		return model.StructureCapabilities{
			RenameTable: true, AddColumn: true, DropColumn: true, AlterColumn: true,
			CreateIndex: true, DropIndex: true, AddConstraint: true, DropConstraint: true,
			PartialIndex: false, IndexMethod: false,
		}
	case model.DriverSQLite:
		// SQLite cannot alter column definitions or add table constraints; those
		// are reported unsupported rather than offered as broken controls.
		return model.StructureCapabilities{
			RenameTable: true, AddColumn: true, DropColumn: true, AlterColumn: false,
			CreateIndex: true, DropIndex: true, AddConstraint: false, DropConstraint: false,
			PartialIndex: true, IndexMethod: false,
		}
	default:
		return model.StructureCapabilities{}
	}
}

// buildStructureStatements turns a validated change into the engine-specific
// statement(s) to execute. It returns ErrNotImplemented for an operation the
// driver does not support and a changeError for invalid input.
func buildStructureStatements(driver model.Driver, quote string, c StructureChange) ([]string, error) {
	switch c.Operation {
	case OpRenameTable:
		return buildRenameTable(driver, quote, c)
	case OpAddColumn:
		return buildAddColumn(quote, c)
	case OpDropColumn:
		return buildDropColumn(quote, c)
	case OpAlterColumn:
		return buildAlterColumn(driver, quote, c)
	case OpCreateIndex:
		return buildCreateIndex(driver, quote, c)
	case OpDropIndex:
		return buildDropIndex(driver, quote, c)
	case OpAddConstraint:
		return buildAddConstraint(driver, quote, c)
	case OpDropConstraint:
		return buildDropConstraint(driver, quote, c)
	default:
		return nil, invalidChange("unknown structure operation %q", c.Operation)
	}
}

func buildRenameTable(driver model.Driver, quote string, c StructureChange) ([]string, error) {
	if strings.TrimSpace(c.Table) == "" || strings.TrimSpace(c.NewName) == "" {
		return nil, invalidChange("table and new_name are required")
	}
	target := qualifiedTable(quote, c.Schema, c.Table)
	renamed := quoteIdentifier(quote, c.NewName)
	if driver == model.DriverMySQL {
		renamed = qualifiedTable(quote, c.Schema, c.NewName)
		return []string{"RENAME TABLE " + target + " TO " + renamed}, nil
	}
	return []string{"ALTER TABLE " + target + " RENAME TO " + renamed}, nil
}

func buildAddColumn(quote string, c StructureChange) ([]string, error) {
	if c.Column == nil {
		return nil, invalidChange("column is required")
	}
	def, err := columnDDL(quote, *c.Column)
	if err != nil {
		return nil, err
	}
	return []string{"ALTER TABLE " + qualifiedTable(quote, c.Schema, c.Table) + " ADD COLUMN " + def}, nil
}

func buildDropColumn(quote string, c StructureChange) ([]string, error) {
	if strings.TrimSpace(c.ColumnName) == "" {
		return nil, invalidChange("column name is required")
	}
	return []string{"ALTER TABLE " + qualifiedTable(quote, c.Schema, c.Table) +
		" DROP COLUMN " + quoteIdentifier(quote, c.ColumnName)}, nil
}

func buildAlterColumn(driver model.Driver, quote string, c StructureChange) ([]string, error) {
	if c.NewColumn == nil || strings.TrimSpace(c.ColumnName) == "" {
		return nil, invalidChange("column name and new definition are required")
	}
	if driver == model.DriverSQLite {
		return nil, fmt.Errorf("%w: SQLite cannot alter an existing column definition", ErrNotImplemented)
	}
	target := qualifiedTable(quote, c.Schema, c.Table)
	col := quoteIdentifier(quote, c.ColumnName)

	if driver == model.DriverMySQL {
		def, err := columnDDL(quote, *c.NewColumn)
		if err != nil {
			return nil, err
		}
		statements := []string{"ALTER TABLE " + target + " MODIFY COLUMN " + def}
		if c.NewColumn.Name != "" && c.NewColumn.Name != c.ColumnName {
			statements = append(statements, "ALTER TABLE "+target+" RENAME COLUMN "+col+" TO "+quoteIdentifier(quote, c.NewColumn.Name))
		}
		return statements, nil
	}

	// PostgreSQL: one statement per changed attribute.
	var statements []string
	if t := strings.TrimSpace(c.NewColumn.Type); t != "" {
		clean, err := sanitizeFragment(t, "type")
		if err != nil {
			return nil, err
		}
		statements = append(statements, "ALTER TABLE "+target+" ALTER COLUMN "+col+" TYPE "+clean)
	}
	switch c.DefaultAction {
	case "set":
		if c.NewColumn.Default == nil {
			return nil, invalidChange("default value is required when setting a default")
		}
		def, err := defaultClause(*c.NewColumn.Default, c.NewColumn.IsExpression)
		if err != nil {
			return nil, err
		}
		statements = append(statements, "ALTER TABLE "+target+" ALTER COLUMN "+col+" SET DEFAULT "+def)
	case "drop":
		statements = append(statements, "ALTER TABLE "+target+" ALTER COLUMN "+col+" DROP DEFAULT")
	}
	statements = append(statements,
		"ALTER TABLE "+target+" ALTER COLUMN "+col+" "+notNullClause(c.NewColumn.Nullable))
	if c.NewColumn.Name != "" && c.NewColumn.Name != c.ColumnName {
		statements = append(statements, "ALTER TABLE "+target+" RENAME COLUMN "+col+" TO "+quoteIdentifier(quote, c.NewColumn.Name))
	}
	return statements, nil
}

func buildCreateIndex(driver model.Driver, quote string, c StructureChange) ([]string, error) {
	if c.Index == nil || strings.TrimSpace(c.Index.Name) == "" || len(c.Index.Columns) == 0 {
		return nil, invalidChange("index name and at least one column are required")
	}
	var cols []string
	for _, column := range c.Index.Columns {
		if strings.TrimSpace(column.Name) == "" {
			return nil, invalidChange("index column name is required")
		}
		frag := quoteIdentifier(quote, column.Name)
		switch strings.ToLower(column.Direction) {
		case "", "asc":
			// default ordering; omit ASC to match the engines' canonical output
		case "desc":
			frag += " DESC"
		default:
			return nil, invalidChange("index column direction must be asc or desc")
		}
		cols = append(cols, frag)
	}
	var b strings.Builder
	b.WriteString("CREATE ")
	if c.Index.Unique {
		b.WriteString("UNIQUE ")
	}
	b.WriteString("INDEX ")
	b.WriteString(quoteIdentifier(quote, c.Index.Name))
	b.WriteString(" ON ")
	b.WriteString(qualifiedTable(quote, c.Schema, c.Table))
	if method := strings.TrimSpace(c.Index.Method); method != "" {
		if driver != model.DriverPostgres {
			return nil, invalidChange("index method is only supported for PostgreSQL")
		}
		clean, err := sanitizeFragment(method, "method")
		if err != nil {
			return nil, err
		}
		b.WriteString(" USING " + clean)
	}
	b.WriteString(" (" + strings.Join(cols, ", ") + ")")
	if where := strings.TrimSpace(c.Index.Where); where != "" {
		if driver == model.DriverMySQL {
			return nil, invalidChange("partial indexes are not supported for MySQL")
		}
		clean, err := sanitizeFragment(where, "where")
		if err != nil {
			return nil, err
		}
		b.WriteString(" WHERE " + clean)
	}
	return []string{b.String()}, nil
}

func buildDropIndex(driver model.Driver, quote string, c StructureChange) ([]string, error) {
	if strings.TrimSpace(c.IndexName) == "" {
		return nil, invalidChange("index name is required")
	}
	name := quoteIdentifier(quote, c.IndexName)
	if driver == model.DriverMySQL {
		return []string{"DROP INDEX " + name + " ON " + qualifiedTable(quote, c.Schema, c.Table)}, nil
	}
	if c.Schema != "" {
		name = quoteIdentifier(quote, c.Schema) + "." + name
	}
	return []string{"DROP INDEX " + name}, nil
}

func buildAddConstraint(driver model.Driver, quote string, c StructureChange) ([]string, error) {
	if c.Constraint == nil || strings.TrimSpace(c.Constraint.Type) == "" {
		return nil, invalidChange("constraint type is required")
	}
	if driver == model.DriverSQLite {
		return nil, fmt.Errorf("%w: SQLite cannot add a table constraint", ErrNotImplemented)
	}
	con := c.Constraint
	prefix := "ALTER TABLE " + qualifiedTable(quote, c.Schema, c.Table) + " ADD "
	if name := strings.TrimSpace(con.Name); name != "" {
		prefix += "CONSTRAINT " + quoteIdentifier(quote, name) + " "
	}

	switch strings.ToUpper(strings.TrimSpace(con.Type)) {
	case "PRIMARY KEY", "UNIQUE":
		if len(con.Columns) == 0 {
			return nil, invalidChange("constraint requires at least one column")
		}
		cols, err := quoteColumns(quote, con.Columns)
		if err != nil {
			return nil, err
		}
		return []string{prefix + strings.ToUpper(strings.TrimSpace(con.Type)) + " (" + cols + ")"}, nil
	case "CHECK":
		clean, err := sanitizeFragment(con.Expression, "expression")
		if err != nil || clean == "" {
			if err == nil {
				err = invalidChange("CHECK constraint requires an expression")
			}
			return nil, err
		}
		return []string{prefix + "CHECK (" + clean + ")"}, nil
	case "FOREIGN KEY":
		return buildForeignKey(quote, prefix, con)
	default:
		return nil, invalidChange("unsupported constraint type %q", con.Type)
	}
}

func buildForeignKey(quote, prefix string, con *ConstraintDefinition) ([]string, error) {
	if len(con.Columns) == 0 || con.Reference == nil || strings.TrimSpace(con.Reference.Table) == "" {
		return nil, invalidChange("foreign key requires source columns and a referenced table")
	}
	if len(con.Columns) != len(con.Reference.Columns) {
		return nil, invalidChange("foreign key source and referenced columns must match in count")
	}
	src, err := quoteColumns(quote, con.Columns)
	if err != nil {
		return nil, err
	}
	ref, err := quoteColumns(quote, con.Reference.Columns)
	if err != nil {
		return nil, err
	}
	stmt := prefix + "FOREIGN KEY (" + src + ") REFERENCES " +
		qualifiedTable(quote, con.Reference.Schema, con.Reference.Table) + " (" + ref + ")"
	if action := normalizeReferentialAction(con.Reference.OnUpdate); action != "" {
		stmt += " ON UPDATE " + action
	}
	if action := normalizeReferentialAction(con.Reference.OnDelete); action != "" {
		stmt += " ON DELETE " + action
	}
	return []string{stmt}, nil
}

func buildDropConstraint(driver model.Driver, quote string, c StructureChange) ([]string, error) {
	if strings.TrimSpace(c.ConstraintName) == "" {
		return nil, invalidChange("constraint name is required")
	}
	if driver == model.DriverSQLite {
		return nil, fmt.Errorf("%w: SQLite cannot drop a table constraint", ErrNotImplemented)
	}
	target := qualifiedTable(quote, c.Schema, c.Table)
	name := quoteIdentifier(quote, c.ConstraintName)
	if driver == model.DriverMySQL {
		switch strings.ToUpper(strings.TrimSpace(c.ConstraintType)) {
		case "FOREIGN KEY":
			return []string{"ALTER TABLE " + target + " DROP FOREIGN KEY " + name}, nil
		case "PRIMARY KEY":
			return []string{"ALTER TABLE " + target + " DROP PRIMARY KEY"}, nil
		case "CHECK":
			return []string{"ALTER TABLE " + target + " DROP CHECK " + name}, nil
		case "UNIQUE":
			return []string{"ALTER TABLE " + target + " DROP INDEX " + name}, nil
		default:
			return nil, invalidChange("constraint type is required for MySQL (PRIMARY KEY | UNIQUE | CHECK | FOREIGN KEY)")
		}
	}
	return []string{"ALTER TABLE " + target + " DROP CONSTRAINT " + name}, nil
}

// columnDDL renders `"name" type [NOT NULL] [DEFAULT ...]`.
func columnDDL(quote string, col ColumnDefinition) (string, error) {
	if strings.TrimSpace(col.Name) == "" || strings.TrimSpace(col.Type) == "" {
		return "", invalidChange("column name and type are required")
	}
	typ, err := sanitizeFragment(col.Type, "type")
	if err != nil {
		return "", err
	}
	parts := []string{quoteIdentifier(quote, col.Name), typ}
	if !col.Nullable {
		parts = append(parts, "NOT NULL")
	}
	if col.Default != nil {
		def, err := defaultClause(*col.Default, col.IsExpression)
		if err != nil {
			return "", err
		}
		parts = append(parts, "DEFAULT "+def)
	}
	return strings.Join(parts, " "), nil
}

// defaultClause renders a literal as a safe SQL literal or an expression as a
// validated raw fragment.
func defaultClause(value string, isExpression bool) (string, error) {
	if isExpression {
		clean, err := sanitizeFragment(value, "default expression")
		if err != nil {
			return "", err
		}
		if clean == "" {
			return "", invalidChange("default expression must not be empty")
		}
		return clean, nil
	}
	trimmed := strings.TrimSpace(value)
	if strings.EqualFold(trimmed, "null") {
		return "NULL", nil
	}
	if isNumericLiteral(trimmed) {
		return trimmed, nil
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'", nil
}

func notNullClause(nullable bool) string {
	if nullable {
		return "DROP NOT NULL"
	}
	return "SET NOT NULL"
}

func quoteColumns(quote string, columns []string) (string, error) {
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		if strings.TrimSpace(column) == "" {
			return "", invalidChange("column name must not be empty")
		}
		quoted = append(quoted, quoteIdentifier(quote, column))
	}
	return strings.Join(quoted, ", "), nil
}

func normalizeReferentialAction(action string) string {
	switch strings.ToUpper(strings.TrimSpace(action)) {
	case "CASCADE", "RESTRICT", "NO ACTION", "SET NULL", "SET DEFAULT":
		return strings.ToUpper(strings.TrimSpace(action))
	default:
		return ""
	}
}

func isNumericLiteral(s string) bool {
	if s == "" {
		return false
	}
	dot := false
	start := 0
	if s[0] == '+' || s[0] == '-' {
		start = 1
	}
	if start == len(s) {
		return false
	}
	for i := start; i < len(s); i++ {
		switch {
		case s[i] >= '0' && s[i] <= '9':
		case s[i] == '.' && !dot:
			dot = true
		default:
			return false
		}
	}
	return true
}

// sanitizeFragment rejects raw SQL fragments that could chain statements or
// comment out the remainder (semicolons, comments, NUL). Type names, expressions
// and predicates are inherently SQL text; this keeps each request a single
// statement built from quoted identifiers plus validated fragments.
func sanitizeFragment(fragment, field string) (string, error) {
	trimmed := strings.TrimSpace(fragment)
	if trimmed == "" {
		return "", nil
	}
	if strings.ContainsRune(trimmed, 0) ||
		strings.Contains(trimmed, ";") ||
		strings.Contains(trimmed, "--") ||
		strings.Contains(trimmed, "/*") ||
		strings.Contains(trimmed, "*/") {
		return "", invalidChange("%s contains an unsupported character or comment", field)
	}
	return trimmed, nil
}

// ApplyStructure builds and executes one schema change for the (id, database)
// target. PostgreSQL/SQLite changes run in a single transaction (all-or-nothing);
// MySQL DDL implicitly commits, so multi-statement changes are rejected rather
// than silently applied partially. A lost connection evicts the pool.
func (m *Manager) ApplyStructure(ctx context.Context, id string, cfg Config, change StructureChange) (int, error) {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	statements, err := buildStructureStatements(cfg.Driver, connector.Capabilities().IdentifierQuote, change)
	if err != nil {
		return 0, err
	}
	if cfg.Driver == model.DriverMySQL && len(statements) > 1 {
		return 0, fmt.Errorf("%w: MySQL cannot apply a multi-statement schema change atomically", ErrNotImplemented)
	}

	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return 0, err
	}

	exec := func(run func(string) error) error {
		for _, statement := range statements {
			if err := run(statement); err != nil {
				return normalizeRestoreError(cfg.Driver, err)
			}
		}
		return nil
	}

	if cfg.Driver == model.DriverMySQL {
		err = exec(func(s string) error { _, e := db.ExecContext(ctx, s); return e })
	} else {
		var tx *sql.Tx
		tx, err = db.BeginTx(ctx, nil)
		if err == nil {
			defer func() { _ = tx.Rollback() }()
			err = exec(func(s string) error { _, e := tx.ExecContext(ctx, s); return e })
			if err == nil {
				err = tx.Commit()
			}
		}
	}
	if err != nil {
		if isConnectionLoss(err) {
			m.evict(id, cfg)
		}
		return 0, err
	}
	return len(statements), nil
}
