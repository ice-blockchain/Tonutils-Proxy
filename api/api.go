package api

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
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
	listener   net.Listener
	config     Config
	state      State
}

func New(cfg Config, state State) (*Server, error) {
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	log.Info().Str("addr", cfg.Addr).Msg("API server listening on")

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		engine:   engine,
		listener: ln,
		config:   cfg,
		state:    state,
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
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return s, nil
}

func (s *Server) Start() error {
	if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
