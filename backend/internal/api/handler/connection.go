// Package handler contains the HTTP handlers for the DataDeck API. Handlers
// validate and translate transport concerns; persistence and target database
// work is delegated to the repository and database packages.
package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
)

const (
	maxBodyBytes          = 1 << 20 // 1 MiB request body limit
	defaultPostgresPort   = 5432
	defaultMySQLPort      = 3306
	testConnectionTimeout = 10 * time.Second
)

// defaultPortFor returns the conventional port for a supported driver.
func defaultPortFor(driver model.Driver) int {
	switch driver {
	case model.DriverMySQL:
		return defaultMySQLPort
	case model.DriverPostgres:
		return defaultPostgresPort
	default:
		return 0
	}
}

// ConnectionHandler serves the connection profile endpoints.
type ConnectionHandler struct {
	connections *repository.ConnectionRepository
	manager     *database.Manager
	cipher      *security.Cipher
	logger      *slog.Logger
}

// NewConnectionHandler constructs a handler for connection profile endpoints.
func NewConnectionHandler(
	connections *repository.ConnectionRepository,
	manager *database.Manager,
	cipher *security.Cipher,
	logger *slog.Logger,
) *ConnectionHandler {
	return &ConnectionHandler{connections: connections, manager: manager, cipher: cipher, logger: logger}
}

// ConnectionRequest is the body for creating a connection profile. The password
// is write-only: it is encrypted before persistence and never returned by any
// endpoint.
//
// PRF-01: for `postgres`, `database_name` is an optional default database (a
// PostgreSQL profile may represent a server/instance); for `mysql` and `sqlite`
// it remains required (database / file path).
type ConnectionRequest struct {
	Name         string `json:"name" binding:"required" example:"Local PG"`
	Driver       string `json:"driver" binding:"required" enums:"postgres,mysql,sqlite" example:"postgres"`
	Host         string `json:"host" example:"127.0.0.1"`
	Port         int    `json:"port" example:"5432"`
	DatabaseName string `json:"database_name" example:"app"`
	Username     string `json:"username" example:"appuser"`
	Password     string `json:"password" example:"secret"`
	SSLMode      string `json:"ssl_mode" enums:"disable,allow,prefer,require,verify-ca,verify-full" example:"disable"`
}

// ConnectionTestRequest is the body for testing connection parameters. It has no
// name because nothing is persisted. `database_name` follows the same
// per-driver optionality as ConnectionRequest.
type ConnectionTestRequest struct {
	Driver       string `json:"driver" binding:"required" enums:"postgres,mysql,sqlite" example:"postgres"`
	Host         string `json:"host" example:"127.0.0.1"`
	Port         int    `json:"port" example:"5432"`
	DatabaseName string `json:"database_name" example:"app"`
	Username     string `json:"username" example:"appuser"`
	Password     string `json:"password" example:"secret"`
	SSLMode      string `json:"ssl_mode" enums:"disable,allow,prefer,require,verify-ca,verify-full" example:"disable"`
}

// ConnectionResponse is the sanitized API representation of a profile. It never
// carries plaintext or encrypted credentials.
type ConnectionResponse struct {
	ID           string    `json:"id" example:"9f1c7d2e4a6b4e89b88ad5f356bf7312"`
	Name         string    `json:"name" example:"Local PG"`
	Driver       string    `json:"driver" enums:"postgres,mysql,sqlite" example:"postgres"`
	Host         *string   `json:"host" example:"127.0.0.1"`
	Port         *int      `json:"port" example:"5432"`
	DatabaseName string    `json:"database_name" example:"app"`
	Username     *string   `json:"username" example:"appuser"`
	SSLMode      string    `json:"ssl_mode" example:"disable"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func newConnectionResponse(profile model.ConnectionProfile) ConnectionResponse {
	return ConnectionResponse{
		ID:           profile.ID,
		Name:         profile.Name,
		Driver:       string(profile.Driver),
		Host:         profile.Host,
		Port:         profile.Port,
		DatabaseName: profile.DatabaseName,
		Username:     profile.Username,
		SSLMode:      profile.SSLMode,
		CreatedAt:    profile.CreatedAt,
		UpdatedAt:    profile.UpdatedAt,
	}
}

// TestConnectionResponse is returned by the connection test endpoint.
type TestConnectionResponse struct {
	Status string `json:"status" example:"ok"`
}

// DeleteConnectionResponse is returned by the delete endpoint.
type DeleteConnectionResponse struct {
	ID string `json:"id"`
}

// List godoc
// @Summary      List connection profiles
// @Description  Returns all persisted connection profiles with credentials omitted.
// @Tags         connections
// @Produce      json
// @Success      200  {object}  response.Envelope{data=[]handler.ConnectionResponse}
// @Failure      500  {object}  response.ErrorEnvelope
// @Router       /connections [get]
func (h *ConnectionHandler) List(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.connections.List(r.Context())
	if err != nil {
		h.repositoryError(w, r, "list_connections", err)
		return
	}
	out := make([]ConnectionResponse, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, newConnectionResponse(profile))
	}
	response.Success(w, out)
}

// Create godoc
// @Summary      Create a connection profile
// @Description  Validates and persists a profile. The password is encrypted at rest and never returned.
// @Tags         connections
// @Accept       json
// @Produce      json
// @Param        request  body      handler.ConnectionRequest  true  "Connection profile"
// @Success      200      {object}  response.Envelope{data=handler.ConnectionResponse}
// @Failure      400      {object}  response.ErrorEnvelope
// @Failure      500      {object}  response.ErrorEnvelope
// @Router       /connections [post]
func (h *ConnectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req ConnectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	profile, err := normalizeRequest(req.Name, req.Driver, req.Host, req.Port, req.DatabaseName, req.Username, req.SSLMode, true)
	if err != nil {
		response.ValidationError(w, err.Error())
		return
	}

	id, err := newID()
	if err != nil {
		h.internalError(w, r, "generate_id", err)
		return
	}
	profile.ID = id

	if req.Password != "" {
		encrypted, err := h.cipher.Encrypt([]byte(req.Password))
		if err != nil {
			h.internalError(w, r, "encrypt_password", err)
			return
		}
		profile.EncryptedPassword = &encrypted
	}

	if err := h.connections.Create(r.Context(), profile); err != nil {
		h.repositoryError(w, r, "create_connection", err)
		return
	}
	response.Success(w, newConnectionResponse(*profile))
}

// Test godoc
// @Summary      Test connection parameters
// @Description  Validates parameters and attempts a temporary connection. Nothing is persisted.
// @Tags         connections
// @Accept       json
// @Produce      json
// @Param        request  body      handler.ConnectionTestRequest  true  "Connection parameters"
// @Success      200      {object}  response.Envelope{data=handler.TestConnectionResponse}
// @Failure      400      {object}  response.ErrorEnvelope
// @Failure      502      {object}  response.ErrorEnvelope
// @Router       /connections/test [post]
func (h *ConnectionHandler) Test(w http.ResponseWriter, r *http.Request) {
	var req ConnectionTestRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	profile, err := normalizeRequest("", req.Driver, req.Host, req.Port, req.DatabaseName, req.Username, req.SSLMode, false)
	if err != nil {
		response.ValidationError(w, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), testConnectionTimeout)
	defer cancel()

	if err := h.manager.Test(ctx, toDatabaseConfig(profile, req.Password)); err != nil {
		h.logger.Warn("connection_test_failed",
			"request_id", chimiddleware.GetReqID(r.Context()),
			"driver", string(profile.Driver),
			"error", err.Error(),
		)
		switch {
		case errors.Is(err, database.ErrNoBootstrapDatabase):
			response.WriteError(w, http.StatusBadGateway, "BOOTSTRAP_DATABASE_UNAVAILABLE",
				"no bootstrap database is reachable; provide a database")
		case errors.Is(err, database.ErrDatabaseNotFound):
			response.WriteError(w, http.StatusBadRequest, "DATABASE_NOT_FOUND",
				"the database does not exist")
		case errors.Is(err, database.ErrDatabaseConnectDenied):
			response.WriteError(w, http.StatusBadRequest, "DATABASE_CONNECT_DENIED",
				"the credentials cannot connect to that database")
		default:
			response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR",
				"failed to connect with the provided parameters")
		}
		return
	}
	response.Success(w, TestConnectionResponse{Status: "ok"})
}

// Delete godoc
// @Summary      Delete a connection profile
// @Description  Closes any active pool and removes the profile.
// @Tags         connections
// @Produce      json
// @Param        id  path      string  true  "Connection id"
// @Success      200  {object}  response.Envelope{data=handler.DeleteConnectionResponse}
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      500  {object}  response.ErrorEnvelope
// @Router       /connections/{id} [delete]
func (h *ConnectionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	if err := h.manager.CloseConnection(id); err != nil {
		h.internalError(w, r, "close_connection", err)
		return
	}
	if err := h.connections.Delete(r.Context(), id); err != nil {
		h.repositoryError(w, r, "delete_connection", err)
		return
	}
	response.Success(w, DeleteConnectionResponse{ID: id})
}

func (h *ConnectionHandler) repositoryError(w http.ResponseWriter, r *http.Request, action string, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
		return
	}
	h.internalError(w, r, action, err)
}

func (h *ConnectionHandler) internalError(w http.ResponseWriter, r *http.Request, action string, err error) {
	writeInternalError(h.logger, w, r, action, err)
}

// decodeJSON reads a single JSON object from the request body into dst.
//
// Policy (consistent across every JSON endpoint):
//   - Content-Type must be application/json. This blocks cross-origin
//     "simple request" CSRF, which cannot set that type without a preflight.
//   - The body is capped at maxBodyBytes; exceeding it yields a clear
//     validation error rather than a decode failure.
//   - Exactly one JSON value is accepted. Trailing values or garbage are
//     rejected.
//   - Unknown fields are ignored (forward-compatible), while wrong types fail.
//
// Failures never echo the body and always return the standard validation
// envelope.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		response.ValidationError(w, "Content-Type must be application/json")
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.ValidationError(w, fmt.Sprintf("request body exceeds the %d byte limit", maxBodyBytes))
			return false
		}
		response.ValidationError(w, "invalid JSON request body")
		return false
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		response.ValidationError(w, "request body must contain a single JSON object")
		return false
	}
	return true
}

// isJSONContentType accepts `application/json`, optionally with parameters
// (e.g. `; charset=utf-8`).
func isJSONContentType(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}

// Sentinel validation failures for database binding (PRF-01).
var (
	errDatabaseRequired = errors.New("database is required for this connection; select one from GET /connections/{id}/databases")
	errDatabaseMismatch = errors.New("database does not match this connection's database")
)

// resolveTargetDatabase determines the effective database for an operation.
//
// PostgreSQL profiles may bind per request/tab; an explicit request database
// always wins, then the profile's default. When neither exists the request is
// rejected (never silently substituted). MySQL/SQLite are bound to the profile's
// database: a differing request value is rejected as a mismatch.
func resolveTargetDatabase(profile *model.ConnectionProfile, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	profileDatabase := strings.TrimSpace(profile.DatabaseName)

	switch profile.Driver {
	case model.DriverPostgres:
		if requested != "" {
			return requested, nil
		}
		if profileDatabase != "" {
			return profileDatabase, nil
		}
		return "", errDatabaseRequired
	default:
		if requested != "" && requested != profileDatabase {
			return "", errDatabaseMismatch
		}
		return profileDatabase, nil
	}
}

// writeResolveDatabaseError maps a resolveTargetDatabase failure to the contract
// error: a missing PostgreSQL database is DATABASE_REQUIRED (documented in
// docs/api-contract.md §5.6), everything else is a validation error (e.g. a
// MySQL/SQLite database mismatch).
func writeResolveDatabaseError(w http.ResponseWriter, err error) {
	if errors.Is(err, errDatabaseRequired) {
		response.WriteError(w, http.StatusBadRequest, "DATABASE_REQUIRED", err.Error())
		return
	}
	response.ValidationError(w, err.Error())
}

// normalizeRequest validates input and maps it into a profile. name is required
// only for the create endpoint (requireName); /test does not persist a name.
func normalizeRequest(name, driver, host string, port int, databaseName, username, sslMode string, requireName bool) (*model.ConnectionProfile, error) {
	name = strings.TrimSpace(name)
	if requireName && name == "" {
		return nil, errors.New("name is required")
	}

	driverValue := model.Driver(strings.TrimSpace(driver))
	if driverValue == "" {
		return nil, errors.New("driver is required")
	}

	switch driverValue {
	case model.DriverPostgres, model.DriverMySQL:
		host = strings.TrimSpace(host)
		if host == "" {
			return nil, errors.New("host is required")
		}

		if port == 0 {
			port = defaultPortFor(driverValue)
		}
		if port < 1 || port > 65535 {
			return nil, errors.New("port must be between 1 and 65535")
		}

		databaseName = strings.TrimSpace(databaseName)
		// PRF-01: a PostgreSQL profile may represent a server/instance, so the
		// database is an optional default. MySQL still binds to one database.
		if databaseName == "" && driverValue != model.DriverPostgres {
			return nil, errors.New("database_name is required")
		}

		username = strings.TrimSpace(username)
		if username == "" {
			return nil, errors.New("username is required")
		}

		sslMode = strings.TrimSpace(sslMode)
		if sslMode == "" {
			sslMode = "disable"
		}
		if !validSSLMode(sslMode) {
			return nil, fmt.Errorf("ssl_mode %q is not valid", sslMode)
		}

		return &model.ConnectionProfile{
			Name:         name,
			Driver:       driverValue,
			Host:         &host,
			Port:         &port,
			DatabaseName: databaseName,
			Username:     &username,
			SSLMode:      sslMode,
		}, nil

	case model.DriverSQLite:
		// SQLite targets a file: no host, port, username or password.
		databasePath := strings.TrimSpace(databaseName)
		if databasePath == "" {
			return nil, errors.New("database_name (file path) is required for sqlite")
		}
		return &model.ConnectionProfile{
			Name:         name,
			Driver:       driverValue,
			DatabaseName: databasePath,
			SSLMode:      "disable",
		}, nil

	default:
		return nil, fmt.Errorf("driver %q is not supported yet; supported drivers: postgres, mysql, sqlite", driverValue)
	}
}

func validSSLMode(mode string) bool {
	switch mode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return true
	default:
		return false
	}
}

func toDatabaseConfig(profile *model.ConnectionProfile, password string) database.Config {
	cfg := database.Config{
		Driver:   profile.Driver,
		Database: profile.DatabaseName,
		Password: password,
		SSLMode:  profile.SSLMode,
	}
	if profile.Host != nil {
		cfg.Host = *profile.Host
	}
	if profile.Port != nil {
		cfg.Port = *profile.Port
	}
	if profile.Username != nil {
		cfg.Username = *profile.Username
	}
	return cfg
}

func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
