package main

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-proxy/api"
	"github.com/xssnick/tonutils-proxy/cmd/proxy-cli/config"
	"github.com/xssnick/tonutils-proxy/proxy"
	"go.uber.org/fx"
)

var GitCommit = "dev"

type zerologPrinter struct{}

func (zerologPrinter) Printf(msg string, args ...interface{}) {
	log.Info().Msgf(strings.TrimRight(msg, "\n"), args...)
}

type CLIConfig struct {
	Addr              string
	APIAddr           string
	Verbosity         int
	BlockHttp         bool
	NetworkConfigPath string
	AuthUser          string
	AuthPass          string
}

func parseCLIConfig() CLIConfig {
	var cfg CLIConfig
	flag.StringVar(&cfg.Addr, "addr", "127.0.0.1:8080", "The addr of the proxy.")
	flag.StringVar(&cfg.APIAddr, "api-addr", "127.0.0.1:8081", "The addr of the API server.")
	flag.IntVar(&cfg.Verbosity, "verbosity", 2, "Debug logs")
	flag.BoolVar(&cfg.BlockHttp, "no-http", false, "Block ordinary http requests")
	flag.StringVar(&cfg.NetworkConfigPath, "global-config", "", "path to ton network config file")
	flag.StringVar(&cfg.AuthUser, "auth-user", "", "Basic auth username for proxy access (optional)")
	flag.StringVar(&cfg.AuthPass, "auth-pass", "", "Basic auth password for proxy access (optional)")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).Level(zerolog.InfoLevel)
	if cfg.Verbosity >= 3 {
		log.Logger = log.Logger.Level(zerolog.DebugLevel)
	}

	return cfg
}

func newFileConfig() (*config.Config, error) {
	return config.LoadConfig("./")
}

func newAPIServer(cliCfg CLIConfig) *api.Server {
	return api.New(
		api.Config{
			Addr:     cliCfg.APIAddr,
			AuthUser: cliCfg.AuthUser,
			AuthPass: cliCfg.AuthPass,
		},
		api.State{
			GetTransport: proxy.GetTransport,
			IsReady:      proxy.IsReady,
		},
	)
}

func registerAPILifecycle(lc fx.Lifecycle, srv *api.Server) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := srv.Start(); err != nil {
					log.Error().Err(err).Msg("API server failed")
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}

func registerProxyLifecycle(lc fx.Lifecycle, cliCfg CLIConfig, fileCfg *config.Config) {
	proxy.AuthUser = cliCfg.AuthUser
	proxy.AuthPass = cliCfg.AuthPass

	var customTunNetCfg *liteclient.GlobalConfig
	if fileCfg.CustomTunnelNetworkConfigPath != "" {
		var err error
		if strings.HasPrefix(fileCfg.CustomTunnelNetworkConfigPath, "http://") || strings.HasPrefix(fileCfg.CustomTunnelNetworkConfigPath, "https://") {
			customTunNetCfg, err = liteclient.GetConfigFromUrl(context.Background(), fileCfg.CustomTunnelNetworkConfigPath)
		} else {
			customTunNetCfg, err = liteclient.GetConfigFromFile(fileCfg.CustomTunnelNetworkConfigPath)
		}
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load custom net config for tun")
		}
	}

	tunnelEnabled := fileCfg.TunnelConfig != nil && fileCfg.TunnelConfig.NodesPoolConfigPath != ""

	var closerCtx context.Context
	var stop context.CancelFunc

	var tunnelCtx context.Context
	if tunnelEnabled {
		var cancel context.CancelFunc
		tunnelCtx, cancel = context.WithCancel(context.Background())
		proxy.OnTunnelStopped = cancel
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			closerCtx, stop = context.WithCancel(context.Background())
			go func() {
				err := proxy.RunProxy(closerCtx, cliCfg.Addr, fileCfg.ADNLKey, nil, "CLI "+GitCommit, cliCfg.BlockHttp, cliCfg.NetworkConfigPath, fileCfg.TunnelConfig, customTunNetCfg)
				if err != nil {
					log.Fatal().Err(err).Msg("proxy failed")
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			stop()
			if tunnelEnabled {
				log.Info().Msg("Committing tunnel payments...")
				<-tunnelCtx.Done()
			}
			return nil
		},
	})
}

func main() {
	cliCfg := parseCLIConfig()

	log.Info().Msg("Version:" + GitCommit)
	if cliCfg.BlockHttp {
		log.Info().Msg("Ordinary HTTP Will be blocked (flag --no-http set)")
	}

	app := fx.New(
		fx.Logger(zerologPrinter{}),
		fx.Supply(cliCfg),
		fx.Provide(newFileConfig),
		fx.Provide(newAPIServer),
		fx.Invoke(registerProxyLifecycle),
		fx.Invoke(registerAPILifecycle),
	)

	app.Run()
}
