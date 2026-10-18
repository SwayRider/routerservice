package server

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
	"github.com/swayrider/routerservice/internal/logic"
	log "github.com/swayrider/swlib/logger"
)

const (
	// maxRejoinPoints bounds the work one request can ask for.
	maxRejoinPoints = 8

	// arrivalHeadingTolerance is how far the road a connector arrives on may
	// deviate from the direction of the original route.
	arrivalHeadingTolerance = 45

	// turnBackDegrees: a connector that leaves more than this far from the
	// rider's direction of travel counts as a turn-back.
	turnBackDegrees = 120

	// turnBackFirstManeuverKm: a U-turn within this distance of the start
	// counts as a turn-back.
	turnBackFirstManeuverKm = 0.15

	// rejoinTieSeconds: a later rejoin point must be better than an earlier
	// one by more than this to win, so the rider rejoins early when it costs
	// about the same.
	rejoinTieSeconds = 15.0
)

// RejoinConfig tunes how a way back onto the original route is weighed
// against a fresh route.
type RejoinConfig struct {
	// UTurnPenaltySeconds is added to the score of a connector that starts
	// with a turn-back.
	UTurnPenaltySeconds float64
	// SlackSeconds and SlackPercent: a rejoin is accepted when it is at most
	// the fresh route's time plus the larger of the two.
	SlackSeconds float64
	SlackPercent float64
	// MaxParallel limits the Valhalla calls made at the same time.
	MaxParallel int
}

// DefaultRejoinConfig returns the defaults.
func DefaultRejoinConfig() RejoinConfig {
	return RejoinConfig{
		UTurnPenaltySeconds: 90,
		SlackSeconds:        120,
		SlackPercent:        20,
		MaxParallel:         6,
	}
}

// SetRejoinConfig replaces the rejoin tuning.
func (s *RouterServer) SetRejoinConfig(c RejoinConfig) {
	if c.MaxParallel < 1 {
		c.MaxParallel = 1
	}
	s.rejoin = c
}

// rejoinCandidate is a connector from the rider to one rejoin point.
type rejoinCandidate struct {
	index     int
	resp      *routerv1.RouteResponse
	seconds   float64 // connector travel time
	remaining float64 // travel time left on the original route after the point
	penalty   float64 // turn-back penalty
}

func (c rejoinCandidate) score() float64 { return c.seconds + c.remaining + c.penalty }

// chooseRejoin picks between the candidates and the fresh route. It returns
// the kind, the index of the winning rejoin point (for RJ_REJOIN) and the
// response that goes back to the rider.
func chooseRejoin(
	cfg RejoinConfig,
	fresh *routerv1.RouteResponse,
	candidates []rejoinCandidate,
) (routerv1.RejoinKind, int, *routerv1.RouteResponse, error) {
	if len(candidates) == 0 {
		if fresh == nil {
			return routerv1.RejoinKind_RJ_FRESH, 0, nil, fmt.Errorf("no route found")
		}
		return routerv1.RejoinKind_RJ_FRESH, 0, fresh, nil
	}

	sorted := append([]rejoinCandidate(nil), candidates...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].index < sorted[j].index })
	best := sorted[0]
	for _, c := range sorted[1:] {
		if c.score() < best.score()-rejoinTieSeconds {
			best = c
		}
	}

	if fresh == nil {
		return routerv1.RejoinKind_RJ_REJOIN, best.index, best.resp, nil
	}
	freshSeconds := fresh.Trip.Summary.Time
	slack := math.Max(cfg.SlackSeconds, freshSeconds*cfg.SlackPercent/100)
	if best.score() <= freshSeconds+slack {
		return routerv1.RejoinKind_RJ_REJOIN, best.index, best.resp, nil
	}
	return routerv1.RejoinKind_RJ_FRESH, 0, fresh, nil
}

// startsWithTurnBack reports whether a route leaves against the rider's
// direction of travel: its first bearing is more than turnBackDegrees away
// from heading, or it opens with a U-turn right at the start.
func startsWithTurnBack(resp *routerv1.RouteResponse, heading *int32) bool {
	if resp == nil || resp.Trip == nil || len(resp.Trip.Legs) == 0 || heading == nil {
		return false
	}
	maneuvers := resp.Trip.Legs[0].Maneuvers
	if len(maneuvers) == 0 {
		return false
	}
	diff := math.Abs(float64((maneuvers[0].BearingAfter-*heading+540)%360 - 180))
	if diff > turnBackDegrees {
		return true
	}
	if len(maneuvers) > 1 && maneuvers[0].Length < turnBackFirstManeuverKm {
		switch maneuvers[1].Type {
		case routerv1.ManeuverType_M_UTURN_RIGHT, routerv1.ManeuverType_M_UTURN_LEFT:
			return true
		}
	}
	return false
}

// connectorRequest is the route from the rider's position to one rejoin
// point, with the options and exclusions of the fresh request. The start
// keeps its heading only when withStartHeading is set; the rejoin point
// always carries the direction of the original route.
func connectorRequest(
	fresh *routerv1.RouteRequest,
	point *routerv1.RejoinPoint,
	withStartHeading bool,
) *routerv1.RouteRequest {
	req := proto.Clone(fresh).(*routerv1.RouteRequest)
	req.Id = nil
	start := req.Locations[0]
	if !withStartHeading {
		start.PreferredHeading = nil
		start.HeadingTolerance = nil
	}
	heading := point.Heading
	tolerance := int32(arrivalHeadingTolerance)
	req.Locations = []*routerv1.RouteLocation{
		start,
		{
			Location:         &pbgeo.Coordinate{Lat: point.Location.Lat, Lon: point.Location.Lon},
			Type:             routerv1.LocationType_L_BREAK,
			PreferredHeading: &heading,
			HeadingTolerance: &tolerance,
		},
	}
	return req
}

func validateRejoin(req *routerv1.RejoinRequest) error {
	if req.Fresh == nil || len(req.Fresh.Locations) < 2 {
		return fmt.Errorf("fresh route needs at least two locations")
	}
	if err := validateLocations(req.Fresh.Locations); err != nil {
		return err
	}
	if len(req.RejoinPoints) > maxRejoinPoints {
		return fmt.Errorf("at most %d rejoin points", maxRejoinPoints)
	}
	for i, p := range req.RejoinPoints {
		if p == nil || p.Location == nil {
			return fmt.Errorf("rejoin point %d: missing coordinate", i)
		}
		if err := validateLocations([]*routerv1.RouteLocation{{Location: p.Location}}); err != nil {
			return fmt.Errorf("rejoin point %d: %w", i, err)
		}
		if p.Heading < 0 || p.Heading > 359 {
			return fmt.Errorf("rejoin point %d: heading out of range", i)
		}
		if p.RemainingSeconds < 0 || math.IsNaN(p.RemainingSeconds) || math.IsInf(p.RemainingSeconds, 0) {
			return fmt.Errorf("rejoin point %d: invalid remaining time", i)
		}
	}
	return nil
}

// Rejoin guides a rider who left a route back onto it: it calculates a
// connector to every rejoin point and a fresh route to the remaining stops,
// and answers with the best way back (see chooseRejoin).
func (s *RouterServer) Rejoin(
	ctx context.Context,
	req *routerv1.RejoinRequest,
) (*routerv1.RejoinResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("Rejoin"))
	if err := validateRejoin(req); err != nil {
		lg.Debugln(err.Error())
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cfg := s.rejoin
	token := incomingToken(ctx)
	startHeading := req.Fresh.Locations[0].PreferredHeading

	sem := make(chan struct{}, cfg.MaxParallel)
	var wg sync.WaitGroup
	run := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			f()
		}()
	}

	var fresh *routerv1.RouteResponse
	run(func() { fresh = s.freshRoute(ctx, req.Fresh, token, lg) })

	candidates := make([]*rejoinCandidate, len(req.RejoinPoints))
	for i, point := range req.RejoinPoints {
		run(func() {
			candidates[i] = s.connector(ctx, req.Fresh, point, i, startHeading, cfg, token, lg)
		})
	}
	wg.Wait()

	found := make([]rejoinCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c != nil {
			found = append(found, *c)
		}
	}

	kind, index, resp, err := chooseRejoin(cfg, fresh, found)
	if err != nil {
		lg.Warnf("rejoin: no way back found: %v", err)
		return nil, grpcStatus(logic.ErrNoRouteFound)
	}
	lg.Infof("rejoin: %d of %d connectors found, fresh=%t, chose %s (index %d)",
		len(found), len(req.RejoinPoints), fresh != nil, kind, index)
	return &routerv1.RejoinResponse{Kind: kind, RejoinIndex: int32(index), Trip: resp.Trip}, nil
}

// freshRoute calculates the fresh route, relaxing the constraints step by
// step when it cannot be found: first without the start heading, then also
// without the exclusions. nil when nothing works.
func (s *RouterServer) freshRoute(
	ctx context.Context,
	fresh *routerv1.RouteRequest,
	token string,
	lg *log.Logger,
) *routerv1.RouteResponse {
	attempts := []*routerv1.RouteRequest{fresh}

	noHeading := proto.Clone(fresh).(*routerv1.RouteRequest)
	noHeading.Locations[0].PreferredHeading = nil
	noHeading.Locations[0].HeadingTolerance = nil
	attempts = append(attempts, noHeading)

	if len(fresh.ExcludePolygons) > 0 || len(fresh.ExcludeLocations) > 0 {
		plain := proto.Clone(noHeading).(*routerv1.RouteRequest)
		plain.ExcludePolygons = nil
		plain.ExcludeLocations = nil
		attempts = append(attempts, plain)
	}

	for _, attempt := range attempts {
		resp, err := s.calculateRoute(ctx, attempt, token, lg)
		if err == nil && resp.Trip != nil && resp.Trip.Summary != nil {
			return resp
		}
		lg.Debugf("rejoin: fresh route attempt failed: %v", err)
	}
	return nil
}

// connector calculates the way from the rider to rejoin point index. With a
// start heading it first asks for a route that leaves in the direction of
// travel; when that fails or still opens with a turn-back, a route without
// the heading is calculated too and the cheaper of the two (turn-back
// penalty included) is used. nil when no connector exists.
func (s *RouterServer) connector(
	ctx context.Context,
	fresh *routerv1.RouteRequest,
	point *routerv1.RejoinPoint,
	index int,
	startHeading *int32,
	cfg RejoinConfig,
	token string,
	lg *log.Logger,
) *rejoinCandidate {
	try := func(withStartHeading bool) *rejoinCandidate {
		resp, err := s.calculateRoute(ctx, connectorRequest(fresh, point, withStartHeading), token, lg)
		if err != nil || resp.Trip == nil || resp.Trip.Summary == nil {
			return nil
		}
		c := &rejoinCandidate{
			index:     index,
			resp:      resp,
			seconds:   resp.Trip.Summary.Time,
			remaining: point.RemainingSeconds,
		}
		if startsWithTurnBack(resp, startHeading) {
			c.penalty = cfg.UTurnPenaltySeconds
		}
		return c
	}

	if startHeading == nil {
		return try(false)
	}
	withHeading := try(true)
	if withHeading != nil && withHeading.penalty == 0 {
		return withHeading
	}
	without := try(false)
	switch {
	case withHeading == nil:
		return without
	case without == nil:
		return withHeading
	case without.score() < withHeading.score():
		return without
	default:
		return withHeading
	}
}
