package logic

import (
	"context"

	"github.com/swayrider/grpcclients/regionclient"
)

// regionQuerier is the subset of regionclient.Client used by the region-
// assignment and border-crossing orchestration, narrowed to an interface so
// it can be substituted with a test double. *regionclient.Client already
// implements this.
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
