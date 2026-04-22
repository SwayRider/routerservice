package server

import (
	"context"
	"errors"
	"net"
	_ "time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	"github.com/swayrider/routerservice/internal/logic"
	"github.com/swayrider/routerservice/internal/valhalla"
	log "github.com/swayrider/swlib/logger"
)

func (s *RouterServer) Route(
	ctx context.Context,
	req *routerv1.RouteRequest,
) (*routerv1.RouteResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("Route"))

	if len(req.Locations) < 2 {
		lg.Debugln("At least two locations must be specified")
		return nil, status.Error(
			codes.InvalidArgument, "No locations specified",
		)
	}

	locationList, regionAssignment, err := s.assignRegionsToLocations(req, lg)
	if err != nil {
		return nil, grpcStatus(err)
	}
	_ = locationList

	opts := s.createRequestOptions(req)
	routingRequests, err := logic.CreateRoutingRequests(
		req.Id, req.Mode, req.Locations, regionAssignment, lg, opts...)
	if err != nil {
		lg.Errorf("failed to create routing requests: %v", err)
		return nil, grpcStatus(err)
	}

	regionList := make([]string, 0, len(routingRequests))
	for _, routeReq := range routingRequests {
		regionList = append(regionList, routeReq.Region)
	}
	vhClient := valhalla.GetClientForRegions(s.valhallaConfig, regionList)

	if len(regionList) > 1 {
		highwayPref := 0.5
		primaryPref := 0.5
		maxPrimary := false
		if req.Mode == routerv1.RoutingMode_RM_MOTORSCOOTER {
			maxPrimary = true
		}

		if req.RouteOptions != nil {
			if req.RouteOptions.HighwayPreference != nil {
				highwayPref = *req.RouteOptions.HighwayPreference
			}
			if req.RouteOptions.PrimaryPreference != nil {
				primaryPref = *req.RouteOptions.PrimaryPreference
			}
		}

		err := routingRequests.AddBorderCrossings(
			s.regionClient, vhClient,
			req.Mode, highwayPref, primaryPref, maxPrimary, lg)
		if err != nil {
			return nil, grpcStatus(err)
		}
	}

	// TODO: Make use of goroutines
	respList := make([]*vhtypes.RouteResponse, 0, len(routingRequests))
	for _, routeReq := range routingRequests {
		//ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		resp, err := vhClient.Route(ctx, routeReq.Region, routeReq.RequestData)
		defer cancel()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) {
				return nil, grpcStatus(logic.ErrValhallaUnavailable)
			}
			return nil, grpcStatus(err)
		}

		respList = append(respList, resp)
	}

	routeResponse, err := s.buildCombinedRouteResponse(respList, lg)
	if err != nil {
		lg.Errorf("failed to build route response: %v", err)
		return nil, grpcStatus(err)
	}

	//lg.Infof("Route possible: %v", routePossible)
	lg.Infof("regionAssignment: %v", regionAssignment)

	return routeResponse, err
}

func (s *RouterServer) assignRegionsToLocations(
	req *routerv1.RouteRequest,
	l *log.Logger,
) (
	locationList []*pbgeo.Coordinate,
	assignmentList []*logic.RegionAssignment,
	err error,
) {
	lg := l.Derive(log.WithFunction("assignRegionsToLocations"))

	locationList = make([]*pbgeo.Coordinate, 0, len(req.Locations))
	for _, loc := range req.Locations {
		locationList = append(locationList, loc.Location)
	}

	var routePossible bool
	assignmentList, routePossible, err = logic.CalculateRegionAssignment(
		s.regionClient,
		locationList,
		lg,
	)
	if err != nil {
		return
	}
	if !routePossible {
		err = logic.ErrNoRouteFound
		return
	}
	return
}

func (s *RouterServer) createRequestOptions(
	req *routerv1.RouteRequest,
) []logic.RoutingRequestOption {
	opts := make([]logic.RoutingRequestOption, 0)

	// Language
	if req.Language != nil {
		opts = append(opts, logic.LanguageOption(*req.Language))
	}

	// Details of instructions
	switch req.ResultMode {
	case routerv1.RoutingResultMode_RRM_NAVIGATION:
		opts = append(opts, logic.RouteDetailsOption(logic.RDFull))
	case routerv1.RoutingResultMode_RRM_DISPLAY_WITH_DETAILS:
		opts = append(opts, logic.RouteDetailsOption(logic.RDDisplayWithDetail))
	case routerv1.RoutingResultMode_RRM_DISPLAY:
		opts = append(opts, logic.RouteDetailsOption(logic.RDDisplay))
	case routerv1.RoutingResultMode_RRM_MINIMAL:
		opts = append(opts, logic.RouteDetailsOption(logic.RDNoInfo))
	}

	// Apply route_type preset defaults (before explicit route_options overrides)
	var routeType routerv1.RouteType
	if req.RouteType != nil {
		routeType = *req.RouteType
	}
	switch routeType {
	case routerv1.RouteType_RT_SCENIC:
		opts = append(opts,
			logic.HighwayPreferenceOption(0.1),
			logic.TrailsPreferenceOption(0.9),
			logic.TollPreferenceOption(0.2),
		)
	case routerv1.RouteType_RT_SHORTEST:
		opts = append(opts,
			logic.ShortestPathOption(true),
			logic.ShortestDistancePreferenceOption(1.0),
		)
		// RT_FASTEST, RT_UNSPECIFIED, default: nothing — Valhalla defaults apply
	}

	if req.RouteOptions != nil {
		// Highway Preference
		if req.RouteOptions.HighwayPreference != nil {
			opts = append(opts, logic.HighwayPreferenceOption(
				*req.RouteOptions.HighwayPreference))
		}

		// Toll Preference
		if req.RouteOptions.TollwayPreference != nil {
			opts = append(opts, logic.TollPreferenceOption(
				*req.RouteOptions.TollwayPreference))
		}

		// Living Street Preference
		if req.RouteOptions.LivingStreetPreference != nil {
			opts = append(opts, logic.LivingStreetsPreferenceOption(
				*req.RouteOptions.LivingStreetPreference))
		}

		// Tracks Preference
		if req.RouteOptions.TrackPreference != nil {
			opts = append(opts, logic.TracksPreferenceOption(
				*req.RouteOptions.TrackPreference))
		}

		// Trails Preference
		if req.RouteOptions.TrailPreference != nil {
			opts = append(opts, logic.TrailsPreferenceOption(
				*req.RouteOptions.TrailPreference))
		}

		// Primary Preference
		if req.RouteOptions.PrimaryPreference != nil {
			opts = append(opts, logic.PrimaryPreferenceOption(
				*req.RouteOptions.PrimaryPreference))
		}

		// Ferry Preference
		if req.RouteOptions.FerryPreference != nil {
			opts = append(opts, logic.FerryPreferenceOption(
				*req.RouteOptions.FerryPreference))
		}

		// Exclude Unpaved
		if req.RouteOptions.ExcludeUnpaved != nil {
			opts = append(opts, logic.ExcludeUnpavedOption(
				*req.RouteOptions.ExcludeUnpaved))
		}

		// Shortest Path
		if req.RouteOptions.ShortestPath != nil {
			opts = append(opts, logic.ShortestPathOption(
				*req.RouteOptions.ShortestPath))
		}

		// Distance Preference
		if req.RouteOptions.DistancePreference != nil {
			opts = append(opts, logic.ShortestDistancePreferenceOption(
				*req.RouteOptions.DistancePreference))
		}
	}

	// Motorcycle preference fields (override route_type presets)
	if req.ScenicPreference != nil {
		sp := float64(*req.ScenicPreference)
		opts = append(opts,
			logic.TrailsPreferenceOption(0.5+sp*0.5),
			logic.FerryPreferenceOption(0.3+sp*0.7),
			logic.HighwayPreferenceOption(1.0-sp*0.5),
		)
	}

	if req.HighwayAvoidance != nil {
		ha := float64(*req.HighwayAvoidance)
		opts = append(opts, logic.HighwayPreferenceOption(1.0-ha))
	}

	if req.TollAvoidance != nil {
		ta := float64(*req.TollAvoidance)
		opts = append(opts, logic.TollPreferenceOption(1.0-ta))
	}

	if req.UnpavedHandling != nil {
		switch *req.UnpavedHandling {
		case routerv1.UnpavedHandling_UH_PREFER:
			opts = append(opts, logic.TracksPreferenceOption(0.9))
		case routerv1.UnpavedHandling_UH_AVOID:
			opts = append(opts, logic.ExcludeUnpavedOption(true))
		}
	}

	return opts
}

func (s *RouterServer) buildCombinedRouteResponse(
	respList []*vhtypes.RouteResponse,
	l *log.Logger,
) (*routerv1.RouteResponse, error) {
	lg := l.Derive(log.WithFunction("buildCombinedRouteResponse"))
	resp, err := buildRouteResponse(respList[0], l)
	if err != nil {
		lg.Errorf("failed to build initial route response: %v", err)
		return nil, err
	}

	for i := 1; i < len(respList); i++ {
		part := respList[i]
		err := addToRouteResponse(resp, part, lg)
		if err != nil {
			lg.Errorf("failed to add part to route response: %v", err)
			return nil, err
		}
	}

	return resp, nil
}

func buildRouteResponse(
	vhResp *vhtypes.RouteResponse,
	l *log.Logger,
) (*routerv1.RouteResponse, error) {
	lg := l.Derive(log.WithFunction("buildRouteResponse"))

	resp := &routerv1.RouteResponse{}

	if vhResp.Id != nil {
		resp.Id = vhResp.Id
	}
	err := addTrip(resp, &vhResp.Trip, false, lg)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func addToRouteResponse(
	resp *routerv1.RouteResponse,
	vhResp *vhtypes.RouteResponse,
	l *log.Logger,
) error {
	lg := l.Derive(log.WithFunction("addToRouteResponse"))
	return addTrip(resp, &vhResp.Trip, true, lg)
}

func addTrip(
	resp *routerv1.RouteResponse,
	trip *vhtypes.Trip,
	appendToResponse bool,
	l *log.Logger,
) error {
	lg := l.Derive(log.WithFunction("addTrip"))

	if !appendToResponse {
		resp.Trip = &routerv1.Trip{}
		resp.Trip.Status = int32(trip.Status)
		resp.Trip.StatusMessage = trip.StatusMessage
		switch trip.Units {
		case vhtypes.Miles:
			resp.Trip.Unit = routerv1.Unit_U_IMPERIAL
		default:
			resp.Trip.Unit = routerv1.Unit_U_METRIC
		}
		resp.Trip.Language = trip.Language.ToString()
	}
	addLocations(resp.Trip, trip.Locations, appendToResponse, lg)
	addLegs(resp.Trip, trip.Legs, appendToResponse, lg)
	createTripSummary(resp.Trip, &trip.Summary, appendToResponse, lg)

	return nil
}

func addLocations(
	trip *routerv1.Trip,
	locations []vhtypes.Location,
	appendToResponse bool,
	l *log.Logger,
) error {
	lg := l.Derive(log.WithFunction("addLocations"))

	if !appendToResponse {
		trip.Locations = make([]*routerv1.RouteLocationReturned, 0, len(locations))
		for _, loc := range locations {
			tripLoc, err := createLocation(&loc, lg)
			if err != nil {
				lg.Errorf("failed to create location: %v", err)
				return err
			}
			trip.Locations = append(trip.Locations, tripLoc)
		}
		return nil
	}

	tmp := make([]*routerv1.RouteLocationReturned, 0, len(trip.Locations)+len(locations)-1)
	tmp = append(tmp, trip.Locations...)
	for i, loc := range locations {
		if i == 0 {
			// Skip the first one, as this is the last one of the previous part
			continue
		}

		tripLoc, err := createLocation(&loc, lg)
		if err != nil {
			lg.Errorf("failed to create location: %v", err)
			return err
		}
		tmp = append(tmp, tripLoc)
	}
	trip.Locations = tmp
	return nil
}

func createLocation(
	vhLoc *vhtypes.Location,
	l *log.Logger,
) (*routerv1.RouteLocationReturned, error) {
	//lg := l.Derive(log.WithFunction("createLocation"))

	loc := &routerv1.RouteLocationReturned{}

	loc.Location = &pbgeo.Coordinate{
		Lat: vhLoc.Lat,
		Lon: vhLoc.Lon,
	}

	loc.Type = routerv1.LocationType_L_BREAK
	if vhLoc.LocationKind != nil {
		switch *vhLoc.LocationKind {
		case vhtypes.Through:
			loc.Type = routerv1.LocationType_L_THROUGH
		case vhtypes.Via:
			loc.Type = routerv1.LocationType_L_VIA
		case vhtypes.BreakThrough:
			loc.Type = routerv1.LocationType_L_BREAK_THROUGH
		}
	}

	if vhLoc.Heading != nil {
		tmp := int32(*vhLoc.Heading)
		loc.PreferredHeading = &tmp
	}

	if vhLoc.TimeZoneOffset != nil {
		loc.TimeZoneOffset = vhLoc.TimeZoneOffset
	}
	if vhLoc.TimeZoneName != nil {
		loc.TimeZoneName = vhLoc.TimeZoneName
	}
	/*if vhLoc.OriginalIndex != nil {
		tmp := int32(*vhLoc.OriginalIndex)
		loc.OriginalIndex = &tmp
	}*/

	if vhLoc.Name != nil {
		if loc.Info == nil {
			loc.Info = &routerv1.LocationInfo{}
		}
		loc.Info.Name = vhLoc.Name
	}
	if vhLoc.City != nil {
		if loc.Info == nil {
			loc.Info = &routerv1.LocationInfo{}
		}
		loc.Info.City = vhLoc.City
	}
	if vhLoc.State != nil {
		if loc.Info == nil {
			loc.Info = &routerv1.LocationInfo{}
		}
		loc.Info.State = vhLoc.State
	}
	if vhLoc.PostalCode != nil {
		if loc.Info == nil {
			loc.Info = &routerv1.LocationInfo{}
		}
		loc.Info.PostalCode = vhLoc.PostalCode
	}
	if vhLoc.Country != nil {
		if loc.Info == nil {
			loc.Info = &routerv1.LocationInfo{}
		}
		loc.Info.Country = vhLoc.Country
	}
	if vhLoc.Phone != nil {
		if loc.Info == nil {
			loc.Info = &routerv1.LocationInfo{}
		}
		loc.Info.Phone = vhLoc.Phone
	}
	if vhLoc.Url != nil {
		if loc.Info == nil {
			loc.Info = &routerv1.LocationInfo{}
		}
		loc.Info.Url = vhLoc.Url
	}

	if vhLoc.SearchFilter != nil {
		loc.Filter = &routerv1.LocationSearchFilter{}
		loc.Filter.ExcludeTunnel = vhLoc.SearchFilter.ExcludeTunnel
		loc.Filter.ExcludeBridge = vhLoc.SearchFilter.ExcludeBridge
		loc.Filter.ExcludeToll = vhLoc.SearchFilter.ExcludeToll
		loc.Filter.ExcludeFerry = vhLoc.SearchFilter.ExcludeFerry
		loc.Filter.ExcludeRamp = vhLoc.SearchFilter.ExcludeRamp
		loc.Filter.ExcludeClosures = vhLoc.SearchFilter.ExcludeClosures

		if vhLoc.SearchFilter.MinRoadClass != nil {
			var tmp routerv1.RoadClass
			ok := true
			switch *vhLoc.SearchFilter.MinRoadClass {
			case vhtypes.Motorway:
				tmp = routerv1.RoadClass_RC_MOTORWAY
			case vhtypes.Trunk:
				tmp = routerv1.RoadClass_RC_TRUNK
			case vhtypes.Primary:
				tmp = routerv1.RoadClass_RC_PRIMARY
			case vhtypes.Secondary:
				tmp = routerv1.RoadClass_RC_SECONDARY
			case vhtypes.Tertiary:
				tmp = routerv1.RoadClass_RC_TERTIARY
			case vhtypes.Unclassified:
				tmp = routerv1.RoadClass_RC_UNCLASSIFIED
			case vhtypes.Residential:
				tmp = routerv1.RoadClass_RC_RESIDENTIAL
			case vhtypes.Service:
				tmp = routerv1.RoadClass_RC_SERVICE
			case vhtypes.Track:
				tmp = routerv1.RoadClass_RC_TRACK
			default:
				ok = false
			}
			if ok {
				loc.Filter.MinRoadClass = &tmp
			}
		}

		if vhLoc.SearchFilter.MaxRoadClass != nil {
			var tmp routerv1.RoadClass
			ok := true
			switch *vhLoc.SearchFilter.MaxRoadClass {
			case vhtypes.Motorway:
				tmp = routerv1.RoadClass_RC_MOTORWAY
			case vhtypes.Trunk:
				tmp = routerv1.RoadClass_RC_TRUNK
			case vhtypes.Primary:
				tmp = routerv1.RoadClass_RC_PRIMARY
			case vhtypes.Secondary:
				tmp = routerv1.RoadClass_RC_SECONDARY
			case vhtypes.Tertiary:
				tmp = routerv1.RoadClass_RC_TERTIARY
			case vhtypes.Unclassified:
				tmp = routerv1.RoadClass_RC_UNCLASSIFIED
			case vhtypes.Residential:
				tmp = routerv1.RoadClass_RC_RESIDENTIAL
			case vhtypes.Service:
				tmp = routerv1.RoadClass_RC_SERVICE
			case vhtypes.Track:
				tmp = routerv1.RoadClass_RC_TRACK
			default:
				ok = false
			}
			if ok {
				loc.Filter.MaxRoadClass = &tmp
			}
		}
	}

	if vhLoc.SideOfStreet != nil {
		var tmp routerv1.SideOfStreet
		ok := true
		switch *vhLoc.SideOfStreet {
		case vhtypes.Left:
			tmp = routerv1.SideOfStreet_SS_LEFT
		case vhtypes.Right:
			tmp = routerv1.SideOfStreet_SS_RIGHT
		default:
			ok = false
		}
		if ok {
			loc.SideOfStreet = &tmp
		}
	}

	if vhLoc.DateTime != nil {
		loc.DateTime = timestamppb.New(*vhLoc.DateTime)
	}

	return loc, nil
}

func addLegs(
	trip *routerv1.Trip,
	legs []vhtypes.Leg,
	appendToResponse bool,
	l *log.Logger,
) error {
	lg := l.Derive(log.WithFunction("addLegs"))

	if !appendToResponse {
		trip.Legs = make([]*routerv1.Leg, 0, len(legs))
		for _, leg := range legs {
			tripLeg, err := createLeg(&leg, lg)
			if err != nil {
				lg.Errorf("failed to create leg: %v", err)
				return err
			}
			trip.Legs = append(trip.Legs, tripLeg)
		}
		return nil
	}

	// We start by extending the last leg (a border crossing does not introduce
	// a new leg)
	tmp := make([]*routerv1.Leg, 0, len(trip.Legs)+len(legs)-1)
	tmp = append(tmp, trip.Legs...)
	for i, leg := range legs {
		if i == 0 {
			// Indicate on the last leg that it should merge with the next leg
			lastLeg := tmp[len(tmp)-1]
			lastLeg.MergeNext = true
		}
		tripLeg, err := createLeg(&leg, lg)
		if err != nil {
			lg.Errorf("failed to create leg: %v", err)
			return err
		}
		tmp = append(tmp, tripLeg)
	}
	trip.Legs = tmp
	return nil
}

func createLeg(
	vhLeg *vhtypes.Leg,
	l *log.Logger,
) (*routerv1.Leg, error) {
	lg := l.Derive(log.WithFunction("createLeg"))

	leg := &routerv1.Leg{}
	leg.Shape = vhLeg.Shape
	leg.ElevationInterval = vhLeg.ElevationInterval

	addManeuvers(leg, vhLeg.Maneuvers, lg)
	addElevation(leg, vhLeg.Elevation, lg)
	createLegSummary(leg, &vhLeg.Summary, lg)

	return leg, nil
}

func addManeuvers(
	leg *routerv1.Leg,
	maneuvers []vhtypes.Maneuver,
	l *log.Logger,
) error {
	lg := l.Derive(log.WithFunction("addManeuvers"))

	leg.Maneuvers = make([]*routerv1.Maneuver, 0, len(maneuvers))
	for _, maneuver := range maneuvers {
		legManeuver, err := createManeuver(&maneuver, l)
		if err != nil {
			lg.Errorf("failed to create maneuver: %v", err)
			return err
		}
		leg.Maneuvers = append(leg.Maneuvers, legManeuver)
	}
	return nil
}

func createManeuver(
	vhManeuver *vhtypes.Maneuver,
	l *log.Logger,
) (*routerv1.Maneuver, error) {
	//lg := l.Derive(log.WithFunction("createManeuver"))

	maneuver := &routerv1.Maneuver{}

	maneuver.Type = routerv1.ManeuverType(vhManeuver.Type)
	maneuver.Instruction = vhManeuver.Instruction
	maneuver.VerbalTransitionAlertInstruction = vhManeuver.VerbalTransitionAlertInstruction
	maneuver.VerbalPreTransitionInstruction = vhManeuver.VerbalPreTransitionInstruction
	maneuver.VerbalPostTransitionInstruction = vhManeuver.VerbalPostTransitionInstruction
	maneuver.StreetNames = vhManeuver.StreetNames
	maneuver.BeginStreetNames = vhManeuver.BeginStreetNames
	maneuver.Time = vhManeuver.Time
	maneuver.Length = vhManeuver.Length
	maneuver.BeginShapeIndex = int32(vhManeuver.BeginShapeIndex)
	maneuver.EndShapeIndex = int32(vhManeuver.EndShapeIndex)
	maneuver.Toll = vhManeuver.Toll
	maneuver.Highway = vhManeuver.Highway
	maneuver.Rough = vhManeuver.Rough
	maneuver.Gate = vhManeuver.Gate
	maneuver.Ferry = vhManeuver.Ferry

	if vhManeuver.Sign != nil {
		maneuver.Sign = &routerv1.Sign{}

		if len(vhManeuver.Sign.ExitNumberElements) > 0 {
			maneuver.Sign.ExitNumberElements = make([]*routerv1.SignElement, len(vhManeuver.Sign.ExitNumberElements))
			for i, elem := range vhManeuver.Sign.ExitNumberElements {
				signElement := &routerv1.SignElement{
					Text: elem.Text,
				}
				if elem.ConsecutiveCount != nil {
					tmp := int32(*elem.ConsecutiveCount)
					signElement.ConsecutiveCount = &tmp
				}
				maneuver.Sign.ExitNumberElements[i] = signElement
			}
		}

		if len(vhManeuver.Sign.ExitBranchElements) > 0 {
			maneuver.Sign.ExitBranchElements = make([]*routerv1.SignElement, len(vhManeuver.Sign.ExitBranchElements))
			for i, elem := range vhManeuver.Sign.ExitBranchElements {
				signElement := &routerv1.SignElement{
					Text: elem.Text,
				}
				if elem.ConsecutiveCount != nil {
					tmp := int32(*elem.ConsecutiveCount)
					signElement.ConsecutiveCount = &tmp
				}
				maneuver.Sign.ExitBranchElements[i] = signElement
			}
		}

		if len(vhManeuver.Sign.ExitTowardElements) > 0 {
			maneuver.Sign.ExitTowardElements = make([]*routerv1.SignElement, len(vhManeuver.Sign.ExitTowardElements))
			for i, elem := range vhManeuver.Sign.ExitTowardElements {
				signElement := &routerv1.SignElement{
					Text: elem.Text,
				}
				if elem.ConsecutiveCount != nil {
					tmp := int32(*elem.ConsecutiveCount)
					signElement.ConsecutiveCount = &tmp
				}
				maneuver.Sign.ExitTowardElements[i] = signElement
			}
		}

		if len(vhManeuver.Sign.ExitNameElements) > 0 {
			maneuver.Sign.ExitNameElements = make([]*routerv1.SignElement, len(vhManeuver.Sign.ExitNameElements))
			for i, elem := range vhManeuver.Sign.ExitNameElements {
				signElement := &routerv1.SignElement{
					Text: elem.Text,
				}
				if elem.ConsecutiveCount != nil {
					tmp := int32(*elem.ConsecutiveCount)
					signElement.ConsecutiveCount = &tmp
				}
				maneuver.Sign.ExitNameElements[i] = signElement
			}
		}
	}

	if vhManeuver.RoundaboutExitCount != nil {
		tmp := int32(*vhManeuver.RoundaboutExitCount)
		maneuver.RoundaboutExitCount = &tmp
	}

	maneuver.DepartInstruction = vhManeuver.DepartInstruction
	maneuver.VerbalDepartInstruction = vhManeuver.VerbalDepartInstruction
	maneuver.ArriveInstruction = vhManeuver.ArriveInstruction
	maneuver.VerbalArriveInstruction = vhManeuver.VerbalArriveInstruction

	if vhManeuver.TransitInfo != nil {
		maneuver.TransitInfo = &routerv1.TransitInfo{}

		maneuver.TransitInfo.OnestopId = vhManeuver.TransitInfo.OnestopId
		maneuver.TransitInfo.ShortName = vhManeuver.TransitInfo.ShortName
		maneuver.TransitInfo.LongName = vhManeuver.TransitInfo.LongName
		maneuver.TransitInfo.Headsign = vhManeuver.TransitInfo.Headsign
		maneuver.TransitInfo.Color = int32(vhManeuver.TransitInfo.Color)
		maneuver.TransitInfo.TextColor = int32(vhManeuver.TransitInfo.TextColor)
		maneuver.TransitInfo.Description = vhManeuver.TransitInfo.Description
		maneuver.TransitInfo.OperatorOnestopId = vhManeuver.TransitInfo.OperatorOnestopId
		maneuver.TransitInfo.OperatorName = vhManeuver.TransitInfo.OperatorName
		maneuver.TransitInfo.OperatorUrl = vhManeuver.TransitInfo.OperatorUrl

		maneuver.TransitInfo.TransitStops = make([]*routerv1.TransitStop, len(vhManeuver.TransitInfo.TransitStops))
		for i, stop := range vhManeuver.TransitInfo.TransitStops {
			stopElement := &routerv1.TransitStop{}

			stopElement.Type = routerv1.TransitStopKind(stop.Type)
			stopElement.Name = stop.Name
			stopElement.ArrivalDateTime = timestamppb.New(stop.ArrivalDateTime)
			stopElement.DepartureDateTime = timestamppb.New(stop.DepartureDateTime)
			stopElement.IsParentStop = stop.IsParentStop
			stopElement.AssumedSchedule = stop.AssumedSchedule
			stopElement.Location = &pbgeo.Coordinate{
				Lat: stop.Lat,
				Lon: stop.Lon,
			}

			maneuver.TransitInfo.TransitStops[i] = stopElement
		}
	}

	maneuver.VerbalMultiCue = vhManeuver.VerbalMultiCue

	switch vhManeuver.TravelMode {
	case vhtypes.PedestrianTravelMode:
		maneuver.TravelMode = routerv1.TravelMode_TM_PEDESTRIAN
	case vhtypes.BicycleTravelMode:
		maneuver.TravelMode = routerv1.TravelMode_TM_BICYCLE
	case vhtypes.TransitTravelMode:
		maneuver.TravelMode = routerv1.TravelMode_TM_TRANSIT
	default:
		maneuver.TravelMode = routerv1.TravelMode_TM_DRIVE
	}

	switch vhManeuver.TravelType {
	case vhtypes.MotorScooterTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_MOTORSCOOTER
	case vhtypes.MotorcycleTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_MOTORCYCLE
	case vhtypes.TruckTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_TRUCK
	case vhtypes.BusTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_BUS
	case vhtypes.FootTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_FOOT
	case vhtypes.WheelchairTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_WHEELCHAIR
	case vhtypes.RoadTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_ROAD
	case vhtypes.HybridTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_HYBRID
	case vhtypes.CrossTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_CROSS
	case vhtypes.MountainTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_MOUNTAIN
	case vhtypes.TramTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_TRAM
	case vhtypes.MetroTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_METRO
	case vhtypes.RailTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_RAIL
	case vhtypes.FerryTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_FERRY
	case vhtypes.CableCarTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_CABLE_CAR
	case vhtypes.GondolaTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_GONDOLA
	case vhtypes.FunicularTravelType:
		maneuver.TravelType = routerv1.TravelType_TT_FUNICULAR
	default:
		maneuver.TravelType = routerv1.TravelType_TT_CAR
	}

	if vhManeuver.BssManeuverType != nil {
		tmp := routerv1.BikeShareManeuver_BS_NONE_ACTION
		switch *vhManeuver.BssManeuverType {
		case vhtypes.RentBikeAtBikeShare:
			tmp = routerv1.BikeShareManeuver_BS_RENT_BIKE_AT_BIKESHARE
		case vhtypes.ReturnBikeAtBikeShare:
			tmp = routerv1.BikeShareManeuver_BS_RETURN_BIKE_AT_BIKESHARE
		}
		maneuver.BssManeuverType = &tmp
	}

	maneuver.BearingBefore = int32(vhManeuver.BearingBefore)
	maneuver.BearingAfter = int32(vhManeuver.BearingAfter)

	if len(vhManeuver.Lanes) > 0 {
		maneuver.Lanes = make([]*routerv1.TurnLane, len(vhManeuver.Lanes))

		for i, lane := range vhManeuver.Lanes {
			laneElement := &routerv1.TurnLane{}

			laneElement.Directions = uint32(lane.Directions)
			if lane.Valid != nil {
				mask := uint32(*lane.Valid)
				laneElement.Valid = &mask
			}
			if lane.Active != nil {
				mask := uint32(*lane.Active)
				laneElement.Active = &mask
			}

			maneuver.Lanes[i] = laneElement
		}
	}

	return maneuver, nil
}

func addElevation(
	leg *routerv1.Leg,
	elevation []float64,
	l *log.Logger,
) error {
	//lg := l.Derive(log.WithFunction("addElevation"))

	leg.Elevation = elevation
	return nil
}

func createTripSummary(
	trip *routerv1.Trip,
	vhSummary *vhtypes.Summary,
	appendToResponse bool,
	l *log.Logger,
) error {
	//lg := l.Derive(log.WithFunction("createSummary"))

	if !appendToResponse {
		trip.Summary = &routerv1.Summary{}

		trip.Summary.Time = vhSummary.Time
		trip.Summary.Length = vhSummary.Length
		trip.Summary.HasToll = vhSummary.HasToll
		trip.Summary.HasHighway = vhSummary.HasHighway
		trip.Summary.HasFerry = vhSummary.HasFerry
		trip.Summary.BoundingBox = &pbgeo.BoundingBox{
			BottomLeft: &pbgeo.Coordinate{
				Lat: vhSummary.MinLat,
				Lon: vhSummary.MinLon,
			},
			TopRight: &pbgeo.Coordinate{
				Lat: vhSummary.MaxLat,
				Lon: vhSummary.MaxLon,
			},
		}
		return nil
	}

	trip.Summary.Time += vhSummary.Time
	trip.Summary.Length += vhSummary.Length
	if vhSummary.HasToll {
		trip.Summary.HasToll = true
	}
	if vhSummary.HasHighway {
		trip.Summary.HasHighway = true
	}
	if vhSummary.HasFerry {
		trip.Summary.HasFerry = true
	}
	if vhSummary.MinLat < trip.Summary.BoundingBox.BottomLeft.Lat {
		trip.Summary.BoundingBox.BottomLeft.Lat = vhSummary.MinLat
	}
	if vhSummary.MinLon < trip.Summary.BoundingBox.BottomLeft.Lon {
		trip.Summary.BoundingBox.BottomLeft.Lon = vhSummary.MinLon
	}
	if vhSummary.MaxLat > trip.Summary.BoundingBox.TopRight.Lat {
		trip.Summary.BoundingBox.TopRight.Lat = vhSummary.MaxLat
	}
	if vhSummary.MaxLon > trip.Summary.BoundingBox.TopRight.Lon {
		trip.Summary.BoundingBox.TopRight.Lon = vhSummary.MaxLon
	}

	return nil
}

func createLegSummary(
	leg *routerv1.Leg,
	vhSummary *vhtypes.Summary,
	l *log.Logger,
) error {
	//lg := l.Derive(log.WithFunction("createSummary"))

	leg.Summary = &routerv1.Summary{}

	leg.Summary.Time = vhSummary.Time
	leg.Summary.Length = vhSummary.Length
	leg.Summary.HasToll = vhSummary.HasToll
	leg.Summary.HasHighway = vhSummary.HasHighway
	leg.Summary.HasFerry = vhSummary.HasFerry
	leg.Summary.BoundingBox = &pbgeo.BoundingBox{
		BottomLeft: &pbgeo.Coordinate{
			Lat: vhSummary.MinLat,
			Lon: vhSummary.MinLon,
		},
		TopRight: &pbgeo.Coordinate{
			Lat: vhSummary.MaxLat,
			Lon: vhSummary.MaxLon,
		},
	}
	return nil
}
