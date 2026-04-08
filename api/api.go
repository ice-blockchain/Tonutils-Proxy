package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xssnick/tonutils-proxy/proxy/transport"
)

type Config struct {
	Addr     string
	AuthUser string
	AuthPass string
}

type State struct {
	GetTransport func() *transport.Transport
	IsReady      func() bool
}

type Server struct {
	engine     *gin.Engine
	httpServer *http.Server
	config     Config
	state      State
}

func New(cfg Config, state State) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		engine: engine,
		config: cfg,
		state:  state,
	}

	// Public endpoints (no auth)
	engine.GET("/health-check", s.handleHealthCheck)
	engine.GET("/metrics", handleMetrics())

	// Resolve endpoint
	resolve := engine.Group("/resolve")
	if cfg.AuthUser != "" && cfg.AuthPass != "" {
		resolve.Use(gin.BasicAuth(gin.Accounts{cfg.AuthUser: cfg.AuthPass}))
	}
	resolve.GET("/:domain", s.handleResolve)

	s.httpServer = &http.Server{
		Addr:    cfg.Addr,
		Handler: engine,
	}

	return s
}

func (s *Server) Start() error {
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
