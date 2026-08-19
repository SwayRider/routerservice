package server

import (
	"context"

	"github.com/swayrider/grpcclients/regionclient"
	healthv1 "github.com/swayrider/protos/health/v1"
	routerv1 "github.com/swayrider/protos/router/v1"
	"github.com/swayrider/routerservice/internal/valhalla"
	log "github.com/swayrider/swlib/logger"
	"github.com/swayrider/swlib/security"
)

func init() {
	security.PublicEndpoint("/health.v1.HealthService/Ping")
	security.PublicEndpoint("/health.v1.HealthService/Check")
	security.UserOrServiceEndpoint("/router.v1.RouterService/Route", []string{"routing:execute"})
}

// regionQuerier is the subset of regionclient.Client used by RouterServer's
// region-assignment/border-crossing orchestration, narrowed to an interface
// so it can be substituted with a test double.
type regionQuerier interface {
	SearchPoint(
		ctx context.Context,
		token string,
		location regionclient.Coordinate,
		includeExtended bool,
	) (regionclient.RegionList, error)

	FindCrossingLocations(
		ctx context.Context,
		token string,
		fromRegion, toRegion string,
		fromLocation, toLocation regionclient.Coordinate,
		config regionclient.BorderCrossingConfig,
		limit int,
	) ([]regionclient.BorderCrossing, error)

	FindRegionPath(
		ctx context.Context,
		token string,
		fromRegion, toRegion string,
	) ([]string, error)

	FindRouteRegionPaths(
		ctx context.Context,
		token string,
		waypoints []regionclient.Coordinate,
		widthKm float64,
	) ([][]string, error)
}

type RouterServer struct {
	routerv1.UnimplementedRouterServiceServer
	valhallaConfig *valhalla.Config
	regionClient   regionQuerier
	l              *log.Logger
}

func NewRouterServer(
	valhallaConfig *valhalla.Config,
	regionClient regionQuerier,
	l *log.Logger,
) *RouterServer {
	return &RouterServer{
		valhallaConfig: valhallaConfig,
		regionClient:   regionClient,
		l: l.Derive(
			log.WithComponent("RouterServer"),
			log.WithFunction("NewRouterServer"),
		),
	}
}

func (s RouterServer) ValhallaConfig() *valhalla.Config {
	return s.valhallaConfig
}

func (s RouterServer) Logger() *log.Logger {
	return s.l
}

// regionPinger is the subset of regionclient.Client that Check depends on,
// narrowed to an interface so it can be substituted with a test double.
type regionPinger interface {
	Ping() error
}

type HealthServer struct {
	healthv1.UnimplementedHealthServiceServer
	regionClient   regionPinger
	valhallaConfig *valhalla.Config
	l              *log.Logger
}

func NewHealthServer(
	regionClient regionPinger,
	valhallaConfig *valhalla.Config,
	l *log.Logger,
) *HealthServer {
	return &HealthServer{
		regionClient:   regionClient,
		valhallaConfig: valhallaConfig,
		l: l.Derive(
			log.WithComponent("HealthServer"),
			log.WithFunction("NewHealthServer"),
		),
	}
}

func (s HealthServer) Logger() *log.Logger {
	return s.l
}
