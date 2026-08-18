package main

import (
	"context"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"github.com/swayrider/grpcclients"
	"github.com/swayrider/grpcclients/authclient"
	"github.com/swayrider/grpcclients/regionclient"
	healthv1 "github.com/swayrider/protos/health/v1"
	routerv1 "github.com/swayrider/protos/router/v1"
	"github.com/swayrider/routerservice/internal/pelias"
	"github.com/swayrider/routerservice/internal/server"
	"github.com/swayrider/routerservice/internal/valhalla"
	"github.com/swayrider/swlib/app"
	"github.com/swayrider/swlib/jwtkeys"
	log "github.com/swayrider/swlib/logger"
)

/*
flags:
	-http-port			(default: 8080)
	-grpc-port			(default: 8081)

	-pelias-prefix		(default: "pelias-")
	-pelias-api-postfix	(default: "-api")
	-pelias-api-port	(default: 3100)
	-pelias-api-region-hosts	(default: ""; e.g. "iberian-peninsula:192.168.1.222,west-europe:192.168.1.222")
	-pelias-api-region-ports	(default: ""; e.g. "iberian-peninsula:33111,west-eurpe:33121")

	-valhalla-prefix	(Default: "valhalla-")
	-valhalla-postfix	(default: "")
	-valhalla-port		(default: 8002)
	-valhalla-region-hosts		(default: ""; e.g. "iberian-peninsula:192.168.1.222,west-europe:192.168.1.222")
	-valhalla-region-ports		(default: ""; e.g. "iberian-peninsula:33001,west-europe:33002")

Environment variables:
	HTTP_PORT
	GRPC_PORT

	PELIAS_PREFIX
	PELIAS_API_POSTFIX
	PELIAS_API_PORT
	PELIAS_API_REGION_HOSTS
	PELIAS_API_REGION_PORTS

	VALHALLA_PREFIX
	VALHALLA_POSTFIX
	VALHALLA_PORT
	VALHALLA_REGION_HOSTS
	VALHALLA_REGION_PORTS
	VALHALLA_TIMEOUT_SECS
*/

const (
	FldPeliasPrefix         = "pelias-prefix"
	FldPeliasApiPostfix     = "pelias-api-postfix"
	FldPeliasApiPort        = "pelias-api-port"
	FldPeliasApiRegionHosts = "pelias-api-region-hosts"
	FldPeliasApiRegionPorts = "pelias-api-region-ports"
	FldValhallaPrefix       = "valhalla-prefix"
	FldValhallaPostfix      = "valhalla-postfix"
	FldValhallaPort         = "valhalla-port"
	FldValhallaRegionHosts  = "valhalla-region-hosts"
	FldValhallaRegionPorts  = "valhalla-region-ports"
	FldValhallaTimeoutSecs  = "valhalla-timeout-secs"

	EnvPeliasPrefix         = "PELIAS_PREFIX"
	EnvPeliasApiPostfix     = "PELIAS_API_POSTFIX"
	EnvPeliasApiPort        = "PELIAS_API_PORT"
	EnvPeliasApiRegionHosts = "PELIAS_API_REGION_HOSTS"
	EnvPeliasApiRegionPorts = "PELIAS_API_REGION_PORTS"
	EnvValhallaPrefix       = "VALHALLA_PREFIX"
	EnvValhallaPostfix      = "VALHALLA_POSTFIX"
	EnvValhallaPort         = "VALHALLA_PORT"
	EnvValhallaRegionHosts  = "VALHALLA_REGION_HOSTS"
	EnvValhallaRegionPorts  = "VALHALLA_REGION_PORTS"
	EnvValhallaTimeoutSecs  = "VALHALLA_TIMEOUT_SECS"

	DefPeliasPrefix       = "pelias-"
	DefPeliasApiPostfix   = "-api"
	DefPeliasApiPort      = 3100
	DefValhallaPrefix     = "valhalla-"
	DefValhallaPostfix    = ""
	DefValhallaPort       = 8002
	DefValhallaTimeoutSecs = 30
)

func main() {
	stdConfigFields :=
		app.BackendServiceFields

	peliasConfig := pelias.NewConfig()
	valhallaConfig := valhalla.NewConfig()

	application := app.New("routerservice").
		WithDefaultConfigFields(stdConfigFields, app.FlagGroupOverrides{}).
		WithServiceClients(
			app.NewServiceClient("authservice", authServiceClientCtor),
			app.NewServiceClient("regionservice", regionServiceClientCtor),
		).
		WithConfigFields(
			app.NewStringConfigField(
				FldPeliasPrefix, EnvPeliasPrefix, "Pelias prefix", DefPeliasPrefix),
			app.NewStringConfigField(
				FldPeliasApiPostfix, EnvPeliasApiPostfix, "Pelias api postfix", DefPeliasApiPostfix),
			app.NewIntConfigField(
				FldPeliasApiPort, EnvPeliasApiPort, "Pelias api port", DefPeliasApiPort),
			app.NewStringArrConfigField(
				FldPeliasApiRegionHosts, EnvPeliasApiRegionHosts, "Pelias api region hosts", []string{}),
			app.NewStringArrConfigField(
				FldPeliasApiRegionPorts, EnvPeliasApiRegionPorts, "Pelias api region ports", []string{}),
			app.NewStringConfigField(
				FldValhallaPrefix, EnvValhallaPrefix, "Valhalla prefix", DefValhallaPrefix),
			app.NewStringConfigField(
				FldValhallaPostfix, EnvValhallaPostfix, "Valhalla postfix", DefValhallaPostfix),
			app.NewIntConfigField(
				FldValhallaPort, EnvValhallaPort, "Valhalla port", DefValhallaPort),
			app.NewStringArrConfigField(
				FldValhallaRegionHosts, EnvValhallaRegionHosts, "Valhalla region hosts", []string{}),
			app.NewStringArrConfigField(
				FldValhallaRegionPorts, EnvValhallaRegionPorts, "Valhalla region ports", []string{}),
			app.NewIntConfigField(
				FldValhallaTimeoutSecs, EnvValhallaTimeoutSecs, "Valhalla request timeout in seconds", DefValhallaTimeoutSecs),
		).
		WithConfigFields(app.RateLimitConfigFields()...).
		WithConfigFields(app.JWTKeysConfigFields()...).
		WithAppData("PeliasConfig", peliasConfig).
		WithAppData("ValhallaConfig", valhallaConfig)

	jwtKeyCache := jwtkeys.New(application.Logger())

	grpcConfig := app.NewGrpcConfig(
		app.AuthInterceptor|app.ClientInfoInterceptor|app.RateLimitInterceptor,
		jwtKeyCache.GetPublicKeys,
		app.GrpcServiceHooks{
			ServiceRegistrar:   grpcRouterRegistrar,
			ServiceHTTPHandler: grpcRouterGateway(application),
		},
		app.GrpcServiceHooks{
			ServiceRegistrar:   grpcHealthRegistrar,
			ServiceHTTPHandler: grpcHealthGateway(application),
		},
	)

	application = application.
		WithBackgroundRoutines(
			app.JWTKeysFetcher(jwtKeyCache),
			app.RateLimitEvictor(grpcConfig),
		).
		WithInitializers(bootstrapFn, app.JWTKeysInitializer(jwtKeyCache), app.RateLimiterInitializer(grpcConfig)).
		WithGrpc(grpcConfig)
	application.Run()
}

func bootstrapFn(a app.App) error {
	lg := a.Logger().Derive(log.WithFunction("bootstrap"))
	lg.Infoln("Bootstrapping service ...")

	var err error

	peliasConfig := app.GetAppData[*pelias.Config](a, "PeliasConfig")

	peliasApiRegionHostsStr := app.GetConfigFieldAsString(a.Config(), FldPeliasApiRegionHosts)
	peliasApiRegionPortsStr := app.GetConfigFieldAsString(a.Config(), FldPeliasApiRegionPorts)
	peliasApiRegionHosts := strings.Split(peliasApiRegionHostsStr, ",")
	peliasApiRegionPorts := strings.Split(peliasApiRegionPortsStr, ",")
	err = peliasConfig.ParseConfig(
		app.GetConfigField[string](a.Config(), FldPeliasPrefix),
		app.GetConfigField[string](a.Config(), FldPeliasApiPostfix),
		app.GetConfigField[int](a.Config(), FldPeliasApiPort),
		peliasApiRegionHosts,
		peliasApiRegionPorts,
	)
	if err != nil {
		return err
	}

	valhallaConfig := app.GetAppData[*valhalla.Config](a, "ValhallaConfig")

	valhallaRegionHostStr := app.GetConfigFieldAsString(a.Config(), FldValhallaRegionHosts)
	valhallaRegionPortStr := app.GetConfigFieldAsString(a.Config(), FldValhallaRegionPorts)
	valhallaRegionHosts := strings.Split(valhallaRegionHostStr, ",")
	valhallaRegionPorts := strings.Split(valhallaRegionPortStr, ",")
	err = valhallaConfig.ParseConfig(
		app.GetConfigField[string](a.Config(), FldValhallaPrefix),
		app.GetConfigField[string](a.Config(), FldValhallaPostfix),
		app.GetConfigField[int](a.Config(), FldValhallaPort),
		valhallaRegionHosts,
		valhallaRegionPorts,
		app.GetConfigField[int](a.Config(), FldValhallaTimeoutSecs),
	)
	if err != nil {
		return err
	}

	return err
}

func regionServiceClientCtor(a app.App) grpcclients.Client {
	lg := a.Logger().Derive(log.WithFunction("regionServiceClientCtor"))
	clnt, err := regionclient.New(
		app.ServiceClientHostAndPort(a, "regionservice"))
	if err != nil {
		lg.Fatalf("failed to create regionservice client: %v", err)
	}
	return clnt
}

func authServiceClientCtor(a app.App) grpcclients.Client {
	lg := a.Logger().Derive(log.WithFunction("authServiceClientCtor"))
	clnt, err := authclient.New(
		app.ServiceClientHostAndPort(a, "authservice"))
	if err != nil {
		lg.Fatalf("failed to create authservice client: %v", err)
	}
	return clnt
}

func grpcRouterRegistrar(r grpc.ServiceRegistrar, a app.App) {
	peliasConfig := app.GetAppData[*pelias.Config](a, "PeliasConfig")
	valhallaConfig := app.GetAppData[*valhalla.Config](a, "ValhallaConfig")
	regionClient := app.GetServiceClient[*regionclient.Client](a, "regionservice")
	srv := server.NewRouterServer(
		peliasConfig,
		valhallaConfig,
		regionClient,
		a.Logger())
	routerv1.RegisterRouterServiceServer(r, srv)
}

func grpcHealthRegistrar(r grpc.ServiceRegistrar, a app.App) {
	srv := server.NewHealthServer(a.Logger())
	healthv1.RegisterHealthServiceServer(r, srv)
}

func grpcRouterGateway(a app.App) app.ServiceHTTPHandler {
	return func(
		ctx context.Context,
		mux *runtime.ServeMux,
		endpoint string,
		opts []grpc.DialOption,
	) error {
		lg := a.Logger().Derive(log.WithFunction("RouterServiceHTTPHandler"))
		if err := routerv1.RegisterRouterServiceHandlerFromEndpoint(
			ctx, mux, endpoint, opts,
		); err != nil {
			lg.Fatalf("failed to register router gRPC gateway: %v", err)
		}
		return nil
	}
}

func grpcHealthGateway(a app.App) app.ServiceHTTPHandler {
	return func(
		ctx context.Context,
		mux *runtime.ServeMux,
		endpoint string,
		opts []grpc.DialOption,
	) error {
		lg := a.Logger().Derive(log.WithFunction("HealthServiceHTTPHandler"))
		if err := healthv1.RegisterHealthServiceHandlerFromEndpoint(
			ctx, mux, endpoint, opts,
		); err != nil {
			lg.Fatalf("failed to register health gRPC gateway: %v", err)
		}
		return nil
	}
}
