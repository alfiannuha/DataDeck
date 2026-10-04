// Package api assembles the HTTP router and route handlers.
package api

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/handler"
	"github.com/datadeck/datadeck/backend/internal/api/middleware"
	"github.com/datadeck/datadeck/backend/internal/config"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
	"github.com/datadeck/datadeck/backend/internal/web"
)

// Server holds the dependencies shared by the HTTP handlers.
type Server struct {
	cfg          config.Config
	logger       *slog.Logger
	credentials  *security.Cipher
	connections  *repository.ConnectionRepository
	history      *repository.QueryHistoryRepository
	savedQueries *repository.SavedQueryRepository
	manager      *database.Manager
}

// NewServer constructs a Server from its dependencies.
func NewServer(
	cfg config.Config,
	logger *slog.Logger,
	credentials *security.Cipher,
	connections *repository.ConnectionRepository,
	history *repository.QueryHistoryRepository,
	savedQueries *repository.SavedQueryRepository,
	manager *database.Manager,
) *Server {
	return &Server{
		cfg:          cfg,
		logger:       logger,
		credentials:  credentials,
		connections:  connections,
		history:      history,
		savedQueries: savedQueries,
		manager:      manager,
	}
}

// Router returns the fully assembled HTTP handler with baseline middleware.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(s.logger))
	r.Use(middleware.Recoverer(s.logger))
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(s.cfg.CORSAllowedOrigins))

	connections := handler.NewConnectionHandler(s.connections, s.manager, s.credentials, s.logger)
	queries := handler.NewQueryHandler(s.connections, s.history, s.manager, s.credentials, s.logger)
	savedQueries := handler.NewSavedQueryHandler(s.savedQueries, s.logger)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.handleHealth)

		r.Get("/connections", connections.List)
		r.Post("/connections", connections.Create)
		r.Post("/connections/test", connections.Test)
		r.Delete("/connections/{id}", connections.Delete)
		r.Get("/connections/{id}/schemas", connections.Schemas)
		r.Get("/connections/{id}/databases", connections.Databases)
		r.Get("/connections/{id}/table-data", connections.TableData)

		r.Post("/query/execute", queries.Execute)
		r.Get("/query/history", queries.History)

		r.Get("/queries/saved", savedQueries.List)
		r.Post("/queries/saved", savedQueries.Create)
		r.Get("/queries/saved/{id}", savedQueries.Get)
		r.Put("/queries/saved/{id}", savedQueries.Update)
		r.Delete("/queries/saved/{id}", savedQueries.Delete)
	})

	// Static frontend is the fallback for every non-API path. API routes above
	// always win; /api* is answered with the JSON envelope by the static handler.
	static := web.NewHandler(mustFrontend(), s.logger)
	if !static.Embedded() {
		s.logger.Warn("frontend_not_embedded",
			slog.String("note", "static UI assets are unavailable in this build; run scripts/build-embedded.sh for a release build"),
		)
	}
	r.NotFound(static.ServeHTTP)

	return r
}

// mustFrontend returns the embedded frontend filesystem (nil when absent).
func mustFrontend() fs.FS {
	frontend, _ := web.Frontend()
	return frontend
}
