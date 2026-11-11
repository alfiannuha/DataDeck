package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

// TableStructure godoc
// @Summary      Get table structure
// @Description  Returns the read-only structure of one table or view (columns, primary key, indexes, constraints, foreign keys) for the explicit connection + database + schema + table. It never reads a row of data and never mutates anything.
// @Tags         table-structure
// @Produce      json
// @Param        id        path     string  true  "Connection id"
// @Param        database  query    string  false  "Target database (required for server-level PostgreSQL)"
// @Param        schema    query    string  false  "Schema (required for PostgreSQL)"
// @Param        table     query    string  true   "Table or view name"
// @Success      200  {object}  response.Envelope{data=model.TableStructure}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      501  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-structure [get]
// TableStructure reads metadata for only the requested relation through the
// driver's bounded StructureReader. No identifier from the client is ever
// concatenated into the catalog queries; all values are bound parameters.
func (h *ConnectionHandler) TableStructure(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	table := strings.TrimSpace(r.URL.Query().Get("table"))
	if table == "" {
		response.ValidationError(w, "table is required")
		return
	}
	schema := strings.TrimSpace(r.URL.Query().Get("schema"))

	profile, err := h.connections.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
			return
		}
		h.internalError(w, r, "get_connection", err)
		return
	}

	if profile.Driver == model.DriverPostgres && schema == "" {
		response.ValidationError(w, "schema is required for postgresql table structure")
		return
	}

	password := ""
	if profile.EncryptedPassword != nil {
		plaintext, err := h.cipher.Decrypt(*profile.EncryptedPassword)
		if err != nil {
			h.internalError(w, r, "decrypt_password", err)
			return
		}
		password = string(plaintext)
	}

	effectiveDatabase, err := resolveTargetDatabase(profile, r.URL.Query().Get("database"))
	if err != nil {
		writeResolveDatabaseError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), introspectionTimeout)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase

	structure, err := h.manager.TableStructure(ctx, id, cfg, schema, table)
	if err != nil {
		h.tableStructureError(w, r, err)
		return
	}
	response.Success(w, structure)
}

// tableStructureError maps structure failures to sanitized API errors. Metadata
// failures reuse the introspection taxonomy (INTROSPECTION_TIMEOUT /
// INTROSPECTION_ERROR); a missing relation is a 404. No driver DSN or SQL is
// ever returned.
func (h *ConnectionHandler) tableStructureError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrTableNotFound):
		response.WriteError(w, http.StatusNotFound, "TABLE_NOT_FOUND", "the requested table was not found")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "unable to connect to the database")
	default:
		h.introspectionError(w, r, err)
	}
}

// StructureColumnRequest is a column spec for add_column / alter_column.
type StructureColumnRequest struct {
	Name         string  `json:"name" example:"status"`
	Type         string  `json:"type" example:"varchar(32)"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default,omitempty" example:"active"`
	IsExpression bool    `json:"is_expression,omitempty"`
}

// StructureIndexColumnRequest is one key part of an index.
type StructureIndexColumnRequest struct {
	Name      string `json:"name" example:"created_at"`
	Direction string `json:"direction,omitempty" enums:"asc,desc"`
}

// StructureIndexRequest describes an index to create.
type StructureIndexRequest struct {
	Name    string                        `json:"name" example:"idx_orders_created"`
	Unique  bool                          `json:"unique,omitempty"`
	Method  string                        `json:"method,omitempty" example:"btree"`
	Columns []StructureIndexColumnRequest `json:"columns"`
	Where   string                        `json:"where,omitempty"`
}

// StructureReferenceRequest is the target of a foreign key.
type StructureReferenceRequest struct {
	Schema   string   `json:"schema,omitempty"`
	Table    string   `json:"table" example:"customers"`
	Columns  []string `json:"columns" example:"id"`
	OnUpdate string   `json:"on_update,omitempty" enums:"NO ACTION,RESTRICT,CASCADE,SET NULL,SET DEFAULT"`
	OnDelete string   `json:"on_delete,omitempty" enums:"NO ACTION,RESTRICT,CASCADE,SET NULL,SET DEFAULT"`
}

// StructureConstraintRequest describes a constraint to add.
type StructureConstraintRequest struct {
	Name       string                     `json:"name,omitempty" example:"fk_orders_customer"`
	Type       string                     `json:"type" enums:"PRIMARY KEY,UNIQUE,CHECK,FOREIGN KEY" example:"FOREIGN KEY"`
	Columns    []string                   `json:"columns,omitempty"`
	Expression string                     `json:"expression,omitempty"`
	Reference  *StructureReferenceRequest `json:"reference,omitempty"`
}

// StructureChangeRequest is the body for POST /connections/{id}/table-structure
// (PRF-04/Phase 2). Exactly one operation is applied; only the fields for that
// operation are read.
type StructureChangeRequest struct {
	Operation      string                      `json:"operation" binding:"required" enums:"rename_table,add_column,drop_column,alter_column,create_index,drop_index,add_constraint,drop_constraint" example:"add_column"`
	Database       string                      `json:"database,omitempty"`
	Schema         string                      `json:"schema,omitempty"`
	Table          string                      `json:"table" binding:"required" example:"orders"`
	NewName        string                      `json:"new_name,omitempty" example:"orders_v2"`
	Column         *StructureColumnRequest     `json:"column,omitempty"`
	ColumnName     string                      `json:"column_name,omitempty" example:"status"`
	NewColumn      *StructureColumnRequest     `json:"new_column,omitempty"`
	DefaultAction  string                      `json:"default_action,omitempty" enums:"set,drop"`
	Index          *StructureIndexRequest      `json:"index,omitempty"`
	IndexName      string                      `json:"index_name,omitempty"`
	Constraint     *StructureConstraintRequest `json:"constraint,omitempty"`
	ConstraintName string                      `json:"constraint_name,omitempty"`
	ConstraintType string                      `json:"constraint_type,omitempty"`
}

// StructureChangeResult reports how many statements were applied.
type StructureChangeResult struct {
	Operation  string `json:"operation"`
	Statements int    `json:"statements"`
}

// ApplyStructure godoc
// @Summary      Apply one schema change to a table
// @Description  Applies exactly one schema-editing operation (rename table, add/alter/drop column, create/drop index, add/drop constraint) to the explicit connection + database + schema + table (PRF-04/Phase 2). Identifiers are quoted with the engine's identifier quote; literal defaults are emitted as SQL literals; raw fragments (type/expression/where) are validated to keep each request a single statement. PostgreSQL/SQLite run all-or-nothing in a transaction; MySQL DDL cannot roll back, so multi-statement changes are rejected. Operations the engine cannot perform return 501 NOT_IMPLEMENTED.
// @Tags         table-structure
// @Accept       json
// @Produce      json
// @Param        id       path      string                              true  "Connection id"
// @Param        request  body      handler.StructureChangeRequest      true  "Schema change"
// @Success      200      {object}  response.Envelope{data=handler.StructureChangeResult}
// @Failure      400      {object}  response.ErrorEnvelope
// @Failure      404      {object}  response.ErrorEnvelope
// @Failure      501      {object}  response.ErrorEnvelope
// @Failure      502      {object}  response.ErrorEnvelope
// @Failure      504      {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-structure [post]
func (h *ConnectionHandler) ApplyStructure(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	var req StructureChangeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Operation) == "" {
		response.ValidationError(w, "operation is required")
		return
	}
	if strings.TrimSpace(req.Table) == "" {
		response.ValidationError(w, "table is required")
		return
	}

	profile, err := h.connections.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
			return
		}
		h.internalError(w, r, "get_connection", err)
		return
	}

	schema := strings.TrimSpace(req.Schema)
	if profile.Driver == model.DriverPostgres && schema == "" {
		response.ValidationError(w, "schema is required for postgresql structure changes")
		return
	}

	if !structureOpSupported(database.StructureCapabilitiesFor(profile.Driver), req.Operation) {
		response.WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
			"this schema change is not supported for the connection's database engine")
		return
	}

	password := ""
	if profile.EncryptedPassword != nil {
		plaintext, err := h.cipher.Decrypt(*profile.EncryptedPassword)
		if err != nil {
			h.internalError(w, r, "decrypt_password", err)
			return
		}
		password = string(plaintext)
	}

	effectiveDatabase, err := resolveTargetDatabase(profile, req.Database)
	if err != nil {
		writeResolveDatabaseError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), introspectionTimeout)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase

	applied, err := h.manager.ApplyStructure(ctx, id, cfg, toStructureChange(req, schema))
	if err != nil {
		h.structureChangeError(w, r, err)
		return
	}
	response.Success(w, StructureChangeResult{Operation: req.Operation, Statements: applied})
}

// structureOpSupported reports whether the engine advertises the operation.
func structureOpSupported(caps model.StructureCapabilities, op string) bool {
	switch op {
	case database.OpRenameTable:
		return caps.RenameTable
	case database.OpAddColumn:
		return caps.AddColumn
	case database.OpDropColumn:
		return caps.DropColumn
	case database.OpAlterColumn:
		return caps.AlterColumn
	case database.OpCreateIndex:
		return caps.CreateIndex
	case database.OpDropIndex:
		return caps.DropIndex
	case database.OpAddConstraint:
		return caps.AddConstraint
	case database.OpDropConstraint:
		return caps.DropConstraint
	default:
		return false
	}
}

func toStructureChange(req StructureChangeRequest, schema string) database.StructureChange {
	change := database.StructureChange{
		Operation:      req.Operation,
		Schema:         schema,
		Table:          strings.TrimSpace(req.Table),
		NewName:        strings.TrimSpace(req.NewName),
		ColumnName:     strings.TrimSpace(req.ColumnName),
		DefaultAction:  strings.TrimSpace(req.DefaultAction),
		IndexName:      strings.TrimSpace(req.IndexName),
		ConstraintName: strings.TrimSpace(req.ConstraintName),
		ConstraintType: strings.TrimSpace(req.ConstraintType),
	}
	if req.Column != nil {
		change.Column = toColumnDefinition(*req.Column)
	}
	if req.NewColumn != nil {
		change.NewColumn = toColumnDefinition(*req.NewColumn)
	}
	if req.Index != nil {
		idx := &database.IndexDefinition{
			Name:   strings.TrimSpace(req.Index.Name),
			Unique: req.Index.Unique,
			Method: strings.TrimSpace(req.Index.Method),
			Where:  strings.TrimSpace(req.Index.Where),
		}
		for _, column := range req.Index.Columns {
			idx.Columns = append(idx.Columns, database.IndexColumn{
				Name:      strings.TrimSpace(column.Name),
				Direction: strings.TrimSpace(column.Direction),
			})
		}
		change.Index = idx
	}
	if req.Constraint != nil {
		con := &database.ConstraintDefinition{
			Name:       strings.TrimSpace(req.Constraint.Name),
			Type:       strings.TrimSpace(req.Constraint.Type),
			Columns:    req.Constraint.Columns,
			Expression: strings.TrimSpace(req.Constraint.Expression),
		}
		if req.Constraint.Reference != nil {
			con.Reference = &database.ReferenceDefinition{
				Schema:   strings.TrimSpace(req.Constraint.Reference.Schema),
				Table:    strings.TrimSpace(req.Constraint.Reference.Table),
				Columns:  req.Constraint.Reference.Columns,
				OnUpdate: strings.TrimSpace(req.Constraint.Reference.OnUpdate),
				OnDelete: strings.TrimSpace(req.Constraint.Reference.OnDelete),
			}
		}
		change.Constraint = con
	}
	return change
}

func toColumnDefinition(column StructureColumnRequest) *database.ColumnDefinition {
	return &database.ColumnDefinition{
		Name:         strings.TrimSpace(column.Name),
		Type:         strings.TrimSpace(column.Type),
		Nullable:     column.Nullable,
		Default:      column.Default,
		IsExpression: column.IsExpression,
	}
}

// structureChangeError maps a schema-change failure to the contract taxonomy.
// Client validation is a 400; unsupported operations a 501; database rejections
// reuse the SQL error taxonomy; nothing leaks a DSN.
func (h *ConnectionHandler) structureChangeError(w http.ResponseWriter, r *http.Request, err error) {
	var sqlErr *database.SQLError
	switch {
	case errors.Is(err, database.ErrInvalidStructureChange):
		response.ValidationError(w, err.Error())
	case errors.Is(err, database.ErrNotImplemented):
		response.WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", err.Error())
	case errors.Is(err, database.ErrTableNotFound):
		response.WriteError(w, http.StatusNotFound, "TABLE_NOT_FOUND", "the requested table was not found")
	case errors.As(err, &sqlErr):
		code := "SQL_ERROR"
		if sqlErr.Syntax {
			code = "SQL_SYNTAX_ERROR"
		}
		response.WriteError(w, http.StatusBadRequest, code, sqlErr.Message)
	case errors.Is(err, context.DeadlineExceeded):
		response.WriteError(w, http.StatusGatewayTimeout, "INTROSPECTION_TIMEOUT", "the schema change timed out")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "unable to connect to the database")
	default:
		h.internalError(w, r, "apply_structure", err)
	}
}
