package server

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	routerv1 "github.com/swayrider/protos/router/v1"
	"github.com/swayrider/routerservice/internal/logic"
	log "github.com/swayrider/swlib/logger"
)

// RerouteConfig tunes how much slower a route may be to keep the rider's
// direction of travel and to avoid a suspected closure.
type RerouteConfig struct {
	// MaxExtraSeconds is how much longer a route that follows the rider's
	// direction of travel and/or avoids the excluded areas may take than the
	// best route without those constraints, and still be preferred.
	MaxExtraSeconds float64
	// MaxParallel limits the Valhalla calls made at the same time.
	MaxParallel int
}

// DefaultRerouteConfig returns the defaults: 5 minutes, 6 parallel calls.
func DefaultRerouteConfig() RerouteConfig {
	return RerouteConfig{MaxExtraSeconds: 300, MaxParallel: 6}
}

// SetRerouteConfig replaces the reroute tuning.
func (s *RouterServer) SetRerouteConfig(c RerouteConfig) {
	if c.MaxParallel < 1 {
		c.MaxParallel = 1
	}
	s.reroute = c
}

// rerouteCandidates are the routes calculated for a reroute; nil when a
// variant was not asked for or has no route.
type rerouteCandidates struct {
	// HE follows the rider's direction of travel and avoids the excluded
	// areas.
	HE *routerv1.RouteResponse
	// E avoids the excluded areas.
	E *routerv1.RouteResponse
	// H follows the rider's direction of travel.
	H *routerv1.RouteResponse
	// C has neither constraint: the best route from where the rider is.
	C *routerv1.RouteResponse
}

func routeSeconds(r *routerv1.RouteResponse) float64 { return r.Trip.Summary.Time }

// chooseReroute picks the route that goes back to the rider: the first of HE,
// E, H (avoiding a closure matters more than avoiding a turn-back) that is at
// most MaxExtraSeconds slower than C, else C. Without a C only the existing
// constrained routes remain, in the same order. It returns the name of the
// chosen variant.
func chooseReroute(cfg RerouteConfig, c rerouteCandidates) (string, *routerv1.RouteResponse, error) {
	ordered := []struct {
		name string
		resp *routerv1.RouteResponse
	}{{"HE", c.HE}, {"E", c.E}, {"H", c.H}}

	for _, candidate := range ordered {
		if candidate.resp == nil {
			continue
		}
		if c.C == nil || routeSeconds(candidate.resp) <= routeSeconds(c.C)+cfg.MaxExtraSeconds {
			return candidate.name, candidate.resp, nil
		}
	}
	if c.C != nil {
		return "C", c.C, nil
	}
	return "", nil, fmt.Errorf("no route found")
}

// Reroute calculates a new route for a rider who left their route: from the
// rider's position (first location, with its preferred heading) through the
// remaining stops, avoiding the excluded areas. It prefers a route that goes
// the way the rider is travelling and avoids the excluded areas, unless that is
// much slower than the best route without those constraints (see
// chooseReroute).
func (s *RouterServer) Reroute(
	ctx context.Context,
	req *routerv1.RouteRequest,
) (*routerv1.RouteResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("Reroute"))

	if len(req.Locations) < 2 {
		return nil, status.Error(codes.InvalidArgument, "No locations specified")
	}
	if err := validateLocations(req.Locations); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token := incomingToken(ctx)

	hasHeading := req.Locations[0].PreferredHeading != nil
	hasExclusions := len(req.ExcludePolygons) > 0 || len(req.ExcludeLocations) > 0

	// Nothing to prefer: a plain route.
	if !hasHeading && !hasExclusions {
		return s.calculateRoute(ctx, req, token, lg)
	}

	variant := func(heading, exclusions bool) *routerv1.RouteRequest {
		v := proto.Clone(req).(*routerv1.RouteRequest)
		v.Id = nil
		if !heading {
			v.Locations[0].PreferredHeading = nil
			v.Locations[0].HeadingTolerance = nil
		}
		if !exclusions {
			v.ExcludePolygons = nil
			v.ExcludeLocations = nil
		}
		return v
	}

	cfg := s.reroute
	sem := make(chan struct{}, cfg.MaxParallel)
	var wg sync.WaitGroup
	var candidates rerouteCandidates
	run := func(target **routerv1.RouteResponse, r *routerv1.RouteRequest) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resp, err := s.calculateRoute(ctx, r, token, lg)
			if err != nil || resp.Trip == nil || resp.Trip.Summary == nil {
				lg.Debugf("reroute: variant failed: %v", err)
				return
			}
			*target = resp
		}()
	}

	run(&candidates.C, variant(false, false))
	if hasHeading {
		run(&candidates.H, variant(true, false))
	}
	if hasExclusions {
		run(&candidates.E, variant(false, true))
	}
	if hasHeading && hasExclusions {
		run(&candidates.HE, variant(true, true))
	}
	wg.Wait()

	name, resp, err := chooseReroute(cfg, candidates)
	if err != nil {
		lg.Warnf("reroute: %v", err)
		return nil, grpcStatus(logic.ErrNoRouteFound)
	}
	lg.Infof("reroute: chose %s (HE=%s E=%s H=%s C=%s)", name,
		secondsOf(candidates.HE), secondsOf(candidates.E), secondsOf(candidates.H), secondsOf(candidates.C))
	return resp, nil
}

func secondsOf(r *routerv1.RouteResponse) string {
	if r == nil {
		return "-"
	}
	return fmt.Sprintf("%.0fs", routeSeconds(r))
}
