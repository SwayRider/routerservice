package logic

import (
	"context"
	"fmt"
	_ "time"

	"github.com/paulmach/orb"
	"github.com/swayrider/grpcclients/regionclient"
	routerv1 "github.com/swayrider/protos/router/v1"
	"github.com/swayrider/routerservice/restclients/valhalla"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	log "github.com/swayrider/swlib/logger"
)

type RouteDetailsMode = uint8

const (
	RDNoInfo = vhtypes.RDNoInfo
	RDDisplay = vhtypes.RDDisplay
	RDDisplayWithDetail = vhtypes.RDDisplayWithDetail
	RDFull = vhtypes.RDFull
)

type RoutingRequestOption interface {
	Apply(*vhtypes.RouteRequest, string)
}

type routingRequestOptionImpl struct {
	fn func(*vhtypes.RouteRequest, string)
}

func (o *routingRequestOptionImpl) Apply(r *vhtypes.RouteRequest, model string) {
	o.fn(r, model)
}

func RouteDetailsOption(details RouteDetailsMode) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetRouteDetailsMode(model, details)
		},
	}
}

func TollPreferenceOption(tollPreference float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetTollPreference(model, tollPreference)
		},
	}
}

func FerryPreferenceOption(ferryPreference float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetFerryPreference(model, ferryPreference)
		},
	}
}

func HighwayPreferenceOption(highwayPreference float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetHighwayPreference(model, highwayPreference)
		},
	}
}

func LivingStreetsPreferenceOption(livingStreetsPreference float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetLivingStreetsPreference(model, livingStreetsPreference)
		},
	}
}

func TracksPreferenceOption(tracksPreference float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetTracksPreference(model, tracksPreference)
		},
	}
}

func TrailsPreferenceOption(trailsPreference float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetTrailsPreference(model, trailsPreference)
		},
	}
}

func PrimaryPreferenceOption(primaryPreference float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetPrimaryPreference(model, primaryPreference)
		},
	}
}

func ShortestPathOption(shortestPath bool) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetShortestPath(model, shortestPath)
		},
	}
}

func ShortestDistancePreferenceOption(shortestDistancePreference float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetShortestDistancePreference(model, shortestDistancePreference)
		},
	}
}

func ExcludeUnpavedOption(excludeUnpaved bool) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetExcludeUnpaved(model, excludeUnpaved)
		},
	}
}

func TopSpeedOption(topSpeed float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetTopSpeed(model, topSpeed)
		},
	}
}

func CurvynessOption(curvyness float64) RoutingRequestOption {
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, model string) {
			r.SetCurvyness(model, curvyness)
		},
	}
}

func LanguageOption(language string) RoutingRequestOption {
	lang := vhtypes.Language(language)
	return &routingRequestOptionImpl{
		fn: func(r *vhtypes.RouteRequest, _ string) {
			r.Language = &lang
		},
	}
}

type RoutingRequest struct {
	Region string
	RequestData *vhtypes.RouteRequest
}

func (r *RoutingRequest) AppendBorderCrossing(
	coordinate regionclient.Coordinate,
) (err error) {
	lastLoc := r.RequestData.Locations[len(r.RequestData.Locations)-1]
	borderLoc := lastLoc
	borderLoc.Lon = coordinate.Longitude
	borderLoc.Lat = coordinate.Latitude
	if lastLoc.LocationKind != nil {
		*lastLoc.LocationKind = vhtypes.Through
	}
	r.RequestData.Locations = append(r.RequestData.Locations, borderLoc)
	return nil
}

func (r *RoutingRequest) PrependBorderCrossing(
	coordinate regionclient.Coordinate,
) (err error) {
	firstLoc := r.RequestData.Locations[0]
	borderLoc := firstLoc
	borderLoc.Lon = coordinate.Longitude
	borderLoc.Lat = coordinate.Latitude
	if firstLoc.LocationKind != nil {
		*firstLoc.LocationKind = vhtypes.Through
	}
	r.RequestData.Locations = append([]vhtypes.Location{borderLoc}, r.RequestData.Locations...)
	return nil
}

type RoutingRequestList []*RoutingRequest

func CreateRoutingRequests(
	id *string,
	mode routerv1.RoutingMode,
	routeLocations []*routerv1.RouteLocation,
	assignmentList []*RegionAssignment,
	l *log.Logger,
	opts ...RoutingRequestOption,
) (
	requestList RoutingRequestList,
	err error,
) {
	lg := l.Derive(log.WithFunction("createRoutingRequests"))
	_ = lg

	model := costingModel(mode)
	for _, assignment := range assignmentList {
		req := vhtypes.NewRouteRequest(
			model,
		)
		for i := assignment.FromIndex; i <= assignment.ToIndex; i++ {
			routeLoc := routeLocations[i]
			loc := vhtypes.NewLocation(
				routeLoc.Location.Lat,
				routeLoc.Location.Lon,
			)
			loc.SetKind(locationKind(routeLoc.Type))
			req.AddLocation(*loc)
		}

		for _, opt := range opts {
			opt.Apply(req, string(model))
		}
		requestList = append(requestList, &RoutingRequest{
			Region: assignment.Region,
			RequestData: req,
		})
	}

	if id != nil {
		if len(requestList) == 1 {
			requestList[0].RequestData.Id = id
			return
		}

		for i := 0; i < len(requestList); i++ {
			partId := fmt.Sprintf("%s#%d", *id, i+1)
			requestList[i].RequestData.Id = &partId
		}
	}
	return
}

func (lst *RoutingRequestList) AddBorderCrossings(
	regionClnt *regionclient.Client,
	valhallaClnt *valhalla.Client,
	mode routerv1.RoutingMode,
	highwayPreference float64,
	primaryPreference float64,
	maxPrimary bool,
	l *log.Logger,
) (err error) {
	lg := l.Derive(log.WithFunction("addBorderCrossings"))
	if len(*lst) == 1 {
		return
	}

	for i := 1; i < len(*lst); i++ {
		// Requests that need to be stitched
		r1 := (*lst)[i-1]
		r2 := (*lst)[i]

		// Regsions of the corresponding requests
		region1 := r1.Region
		region2 := r2.Region

		// Last location of first request and first location of second request
		l1 := r1.RequestData.Locations[len(r1.RequestData.Locations)-1]
		pt1 := orb.Point{l1.Lon, l1.Lat}
		c1 := regionclient.Coordinate{Longitude: l1.Lon, Latitude: l1.Lat}
		l2 := r2.RequestData.Locations[0]
		pt2 := orb.Point{l2.Lon, l2.Lat}
		c2 := regionclient.Coordinate{Longitude: l2.Lon, Latitude: l2.Lat}

		// RoadTypes of the last location of the first request
		// and the first location of the second request
		rt1 := getRoadType(valhallaClnt, region1, pt1, maxPrimary)
		rt2 := getRoadType(valhallaClnt, region2, pt2, maxPrimary)

		// Definitions for selecting border crossings based on distance of 
		// closes point
		definitions := []regionclient.BorderCrossingDefinition{
			{
				MaxBorderDistance: 0,
				RoadTypeOrder: genericRoadTypeOrder(
					maxPrimary, highwayPreference, primaryPreference),
			},
			{
				MaxBorderDistance: 5000,		// 5 Km
				RoadTypeOrder: closeRoadTypeOrder(
					rt1, rt2,
					maxPrimary, highwayPreference, primaryPreference),
				RoadTypeDelta: 5000,
				DropDistance: 1000,
			},
			{
				MaxBorderDistance: 10000,		// 10 Km
				RoadTypeOrder: mediumRoadTypeOrder(
					maxPrimary, highwayPreference, primaryPreference),
			},
			{
				MaxBorderDistance: 50000, 		// 50 Km
				RoadTypeOrder: farRoadTypeOrder(
					maxPrimary, highwayPreference, primaryPreference),
			},
		}
		config := regionclient.BorderCrossingAdvancedConfig{
			Definitions: definitions,
		}

		// Find top-3 crossings
		var crossings []regionclient.BorderCrossing
		crossings, err = regionClnt.FindCrossingLocations(
			region1, region2, c1, c2, config, 3)
		if err != nil {
			lg.Errorf("Failed to find border crossings: %v", err)
			return
		}

		// Determine the preferred road type for the crossing
		var preferredRoadType regionclient.RoadType
		if maxPrimary {
			if primaryPreference >= 0.5 {
				preferredRoadType = regionclient.RT_PRIMARY
			} else {
				preferredRoadType = regionclient.RT_SECONDARY
			}
		} else {
			if highwayPreference >= 0.5 {
				preferredRoadType = regionclient.RT_MOTORWAY
			} else {
				preferredRoadType = regionclient.RT_TRUNK
			}
		}

		// By default select the first crossing result,
		// unless there is an exact match with the preferred road type
		selectedBc := &crossings[0]
		for _, bc := range crossings {
			if bc.RoadType == preferredRoadType {
				selectedBc = &bc
				break
			}
		}

		if err = r1.AppendBorderCrossing(selectedBc.Location); err != nil {
			lg.Errorf("Failed to append border crossing: %v", err)
			return
		}
		if err = r2.PrependBorderCrossing(selectedBc.Location); err != nil {
			lg.Errorf("Failed to prepend border crossing: %v", err)
			return
		}
	}
	return
}

func costingModel(
	mode routerv1.RoutingMode,
) vhtypes.CostingModel {
	switch mode {
	case routerv1.RoutingMode_RM_CAR:
		return vhtypes.Auto
	case routerv1.RoutingMode_RM_MOTORSCOOTER:
		return vhtypes.MotorScooter
	case routerv1.RoutingMode_RM_MOTORCYCLE:
		return vhtypes.Motorcycle
	default:
		return vhtypes.Auto
	}
}

func locationKind(
	locationType routerv1.LocationType,
) vhtypes.LocationKind {
	switch locationType {
	case routerv1.LocationType_L_THROUGH:
		return vhtypes.Through
	case routerv1.LocationType_L_VIA:
		return vhtypes.Via
	case routerv1.LocationType_L_BREAK_THROUGH:
		return vhtypes.BreakThrough
	default:
		return vhtypes.Break
	}
}

func getRoadType(
	clnt *valhalla.Client,
	region string,
	location orb.Point,
	maxPrimary bool,
) *regionclient.RoadType {
	req := vhtypes.NewLocateRequest(location.Lon(), location.Lat())
	//ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	ctx, cancel := context.WithCancel(context.Background())

	resp, err := clnt.Locate(ctx, region, req)
	defer cancel()
	if err != nil {
		return nil
	}
	if len(resp.Edges) == 0 {
		return nil
	}

	switch resp.Edges[0].Edge.Classification.Classification {
	case vhtypes.Motorway:
		if maxPrimary {
			return nil
		}
		res := regionclient.RT_MOTORWAY
		return &res
	case vhtypes.Trunk:
		if maxPrimary {
			return nil
		}
		res := regionclient.RT_TRUNK
		return &res
	case vhtypes.Primary:
		res := regionclient.RT_PRIMARY
		return &res
	case vhtypes.Secondary:
		res := regionclient.RT_SECONDARY
		return &res
	default:
		return nil
	}
}

func genericRoadTypeOrder(
	maxPrimary bool,
	highwayPreference, primaryPreference float64,
) []regionclient.RoadType {
	if maxPrimary {
		if primaryPreference >= 0.5 {
			return []regionclient.RoadType{
				regionclient.RT_PRIMARY,
				regionclient.RT_SECONDARY,
			}
		}
		return []regionclient.RoadType{
			regionclient.RT_SECONDARY,
			regionclient.RT_PRIMARY,
		}
	}
	if highwayPreference >= 0.5 {
		return []regionclient.RoadType{
			regionclient.RT_MOTORWAY,
			regionclient.RT_TRUNK,
			regionclient.RT_PRIMARY,
		}
	}
	return []regionclient.RoadType{
		regionclient.RT_TRUNK,
		regionclient.RT_PRIMARY,
		regionclient.RT_MOTORWAY,
	}
}

func farRoadTypeOrder(
	maxPrimary bool,
	highwayPreference, primaryPreference float64,
) []regionclient.RoadType {
	if maxPrimary {
		if primaryPreference >= 0.5 {
			return []regionclient.RoadType{
				regionclient.RT_PRIMARY,
				regionclient.RT_SECONDARY,
			}
		}
		return []regionclient.RoadType{
			regionclient.RT_SECONDARY,
			regionclient.RT_PRIMARY,
		}
	}
	if highwayPreference >= 0.5 {
		return []regionclient.RoadType{
			regionclient.RT_MOTORWAY,
			regionclient.RT_TRUNK,
			regionclient.RT_PRIMARY,
		}
	}
	return []regionclient.RoadType{
		regionclient.RT_PRIMARY,
		regionclient.RT_TRUNK,
		regionclient.RT_MOTORWAY,
	}
}

func mediumRoadTypeOrder(
	maxPrimary bool,
	highwayPreference, primaryPreference float64,
) []regionclient.RoadType {
	if maxPrimary {
		if primaryPreference >= 0.5 {
			return []regionclient.RoadType{
				regionclient.RT_PRIMARY,
				regionclient.RT_SECONDARY,
			}
		}
		return []regionclient.RoadType{
			regionclient.RT_SECONDARY,
			regionclient.RT_PRIMARY,
		}
	}
	if highwayPreference >= 0.5 {
		return []regionclient.RoadType{
			regionclient.RT_MOTORWAY,
			regionclient.RT_TRUNK,
			regionclient.RT_PRIMARY,
			regionclient.RT_SECONDARY,
		}
	}
	return []regionclient.RoadType{
		regionclient.RT_PRIMARY,
		regionclient.RT_TRUNK,
		regionclient.RT_MOTORWAY,
		regionclient.RT_SECONDARY,
	}
}

func closeRoadTypeOrder(
	rt1 *regionclient.RoadType,
	rt2 *regionclient.RoadType,
	maxPrimary bool,
	highwayPreference, primaryPreference float64,
) []regionclient.RoadType {
	res := make([]regionclient.RoadType, 0)

	if rt1 != nil && rt2 != nil {
		if rt1.ToCode() > rt2.ToCode() {
			res = append(res, *rt1)
			res = append(res, *rt2)
		} else {
			res = append(res, *rt2)
			res = append(res, *rt1)
		}
		if *rt1 != regionclient.RT_SECONDARY && rt2 != nil && *rt2 != regionclient.RT_SECONDARY {
			res = append(res, regionclient.RT_SECONDARY)
		}
		if primaryPreference >= 0.5 {
			if *rt1 != regionclient.RT_PRIMARY && rt2 != nil && *rt2 != regionclient.RT_PRIMARY {
				res = append(res, regionclient.RT_PRIMARY)
			}
		}
		if !maxPrimary {
			if *rt1 != regionclient.RT_TRUNK && rt2 != nil && *rt2 != regionclient.RT_TRUNK {
				res = append(res, regionclient.RT_TRUNK)
			}
			if *rt1 != regionclient.RT_MOTORWAY && rt2 != nil && *rt2 != regionclient.RT_MOTORWAY {
				res = append(res, regionclient.RT_MOTORWAY)
			}
		}
		if primaryPreference < 0.5 {
			if *rt1 != regionclient.RT_PRIMARY && rt2 != nil && *rt2 != regionclient.RT_PRIMARY {
				res = append(res, regionclient.RT_PRIMARY)
			}
		}
		return res
	}

	var rt *regionclient.RoadType
	if rt1 != nil {
		rt = rt1
	}
	if rt2 != nil {
		rt = rt2
	}

	if rt != nil {
		res = append(res, *rt)
		if *rt != regionclient.RT_SECONDARY {
			res = append(res, regionclient.RT_SECONDARY)
		}
		if primaryPreference >= 0.5 {
			if *rt != regionclient.RT_PRIMARY {
				res = append(res, regionclient.RT_PRIMARY)
			}
		}
		if !maxPrimary {
			if *rt != regionclient.RT_TRUNK {
				res = append(res, regionclient.RT_TRUNK)
			}
			if *rt != regionclient.RT_MOTORWAY {
				res = append(res, regionclient.RT_MOTORWAY)
			}
		}
		if primaryPreference < 0.5 {
			if *rt != regionclient.RT_PRIMARY {
				res = append(res, regionclient.RT_PRIMARY)
			}
		}
		return res
	}

	res = append(res, regionclient.RT_SECONDARY)
	if primaryPreference >= 0.5 {
		res = append(res, regionclient.RT_PRIMARY)
	}
	if !maxPrimary {
		res = append(res, regionclient.RT_TRUNK)
		res = append(res, regionclient.RT_MOTORWAY)
	}
	if primaryPreference < 0.5 {
		res = append(res, regionclient.RT_PRIMARY)
	}
	return res
}


