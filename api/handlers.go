package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/xssnick/tonutils-go/ton/dns"
)

func (s *Server) handleResolve(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing domain name"})
		return
	}

	t := s.state.GetTransport()
	if t == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "proxy not ready"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	result, err := t.ResolveDomain(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain).Msg("resolve failed")
		if errors.Is(err, dns.ErrNoSuchRecord) || strings.Contains(err.Error(), "no site record") {
			c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
		} else {
			c.JSON(http.StatusBadGateway, gin.H{"error": "resolve failed"})
		}
		return
	}

	log.Debug().Str("domain", domain).Str("type", result.Type).Str("adnl_address", result.ADNLAddr).Str("bag_id", result.BagID).Str("ip", result.IP).Int("port", result.Port).Msg("resolve success")

	c.JSON(http.StatusOK, result)
}

func (s *Server) handleHealthCheck(c *gin.Context) {
	if s.state.IsReady() {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false})
	}
}

func handleMetrics() gin.HandlerFunc {
	h := promhttp.Handler()
	return gin.WrapH(h)
}
