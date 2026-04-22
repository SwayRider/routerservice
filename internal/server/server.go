package server

import (
	"github.com/swayrider/grpcclients/regionclient"
	healthv1 "github.com/swayrider/protos/health/v1"
	routerv1 "github.com/swayrider/protos/router/v1"
	"github.com/swayrider/routerservice/internal/pelias"
	"github.com/swayrider/routerservice/internal/valhalla"
	log "github.com/swayrider/swlib/logger"
	"github.com/swayrider/swlib/security"
)

func init() {
	security.PublicEndpoint("/health.v1.HealthService/Ping")
}

type RouterServer struct {
	routerv1.UnimplementedRouterServiceServer
	peliasConfig   *pelias.Config
	valhallaConfig *valhalla.Config
	regionClient   *regionclient.Client
	l              *log.Logger
}

func NewRouterServer(
	peliasConfig *pelias.Config,
	valhallaConfig *valhalla.Config,
	regionClient *regionclient.Client,
	l *log.Logger,
) *RouterServer {
	return &RouterServer{
		peliasConfig:   peliasConfig,
		valhallaConfig: valhallaConfig,
		regionClient:   regionClient,
		l: l.Derive(
			log.WithComponent("RouterServer"),
			log.WithFunction("NewRouterServer"),
		),
	}
}

func (s RouterServer) PeliasConfig() *pelias.Config {
	return s.peliasConfig
}

func (s RouterServer) ValhallaConfig() *valhalla.Config {
	return s.valhallaConfig
}

func (s RouterServer) Logger() *log.Logger {
	return s.l
}

type HealthServer struct {
	healthv1.UnimplementedHealthServiceServer
	l *log.Logger
}

func NewHealthServer(
	l *log.Logger,
) *HealthServer {
	return &HealthServer{
		l: l.Derive(
			log.WithComponent("HealthServer"),
			log.WithFunction("NewHealthServer"),
		),
	}
}

func (s HealthServer) Logger() *log.Logger {
	return s.l
}
