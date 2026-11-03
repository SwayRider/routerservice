package server

import (
	"context"
	"sync"
	"time"

	"github.com/swayrider/routerservice/internal/logic"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	log "github.com/swayrider/swlib/logger"
)

// traceAttributer is the part of the Valhalla client the roundabout lookup
// needs, so it can be replaced in tests.
type traceAttributer interface {
	TraceAttributes(
		ctx context.Context,
		region string,
		request *vhtypes.TraceAttributesRequest,
	) (*vhtypes.TraceAttributesResponse, error)
}

const (
	// Shape points sent before the roundabout and after it, so the matcher
	// sees the roads the roundabout is entered and left by.
	roundaboutShapeLead  = 3
	roundaboutShapeTrail = 2

	// Roundabouts of one route that are looked up at the same time.
	roundaboutLookups = 4

	maneuverRoundaboutEnter = vhtypes.ManeuverKind(26)
	maneuverRoundaboutExit  = vhtypes.ManeuverKind(27)
)

var roundaboutTraceAttributes = []string{
	"edge.roundabout",
	"edge.begin_heading",
	"edge.end_heading",
	"edge.road_class",
	"edge.drive_on_right",
	"node.intersecting_edge.begin_heading",
	"node.intersecting_edge.driveability",
	"node.intersecting_edge.road_class",
}

// addRoundaboutArms fills in the exits of every roundabout of [resp], which
// Valhalla only numbers (the exit to take). The route's own edges give the
// exits up to the one taken; a second lookup over a shape that goes once round
// the roundabout gives the rest. It is best effort and all or nothing: when a
// lookup fails, times out or does not agree with Valhalla's own exit count,
// that maneuver keeps no exits (the app then draws a generic roundabout) and
// the route is returned unchanged.
func addRoundaboutArms(
	ctx context.Context,
	client traceAttributer,
	region string,
	timeout time.Duration,
	resp *vhtypes.RouteResponse,
	l *log.Logger,
) {
	lg := l.Derive(log.WithFunction("addRoundaboutArms"))

	sem := make(chan struct{}, roundaboutLookups)
	var wg sync.WaitGroup
	for li := range resp.Trip.Legs {
		leg := &resp.Trip.Legs[li]
		var shape []float64
		decoded := false
		for mi := range leg.Maneuvers {
			m := &leg.Maneuvers[mi]
			if m.Type != maneuverRoundaboutEnter || m.RoundaboutExitCount == nil {
				continue
			}
			if !decoded {
				flat, _, err := polylineCodec.DecodeFlatCoords(nil, []byte(leg.Shape))
				if err != nil {
					lg.Debugf("roundabout exits: bad leg shape: %v", err)
					break
				}
				shape, decoded = flat, true
			}
			end := m.EndShapeIndex
			if mi+1 < len(leg.Maneuvers) && leg.Maneuvers[mi+1].Type == maneuverRoundaboutExit {
				end = leg.Maneuvers[mi+1].EndShapeIndex
			}
			points := roundaboutShape(shape, m.BeginShapeIndex, end)
			if len(points) < 2 {
				continue
			}
			ring := shapeSlice(shape, m.BeginShapeIndex, m.EndShapeIndex)
			lead := shapeLead(shape, m.BeginShapeIndex)
			exitCount := *m.RoundaboutExitCount

			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				angles, reason := lookUpRoundaboutExits(ctx, client, region, timeout, points, lead, ring, exitCount)
				if reason != "" {
					lg.Infof("roundabout exits: not used (%s)", reason)
					return
				}
				lg.Infof("roundabout exits: %d exits", len(angles))
				m.RoundaboutExitAngles = angles
			}()
		}
	}
	wg.Wait()
}

// lookUpRoundaboutExits returns all exits of a roundabout, or why it could not.
// [points] are the route over the roundabout with a little before and after it,
// [lead] the route just before the roundabout, [ring] the route on it.
func lookUpRoundaboutExits(
	ctx context.Context,
	client traceAttributer,
	region string,
	timeout time.Duration,
	points, lead, ring []vhtypes.ShapePoint,
	exitCount int,
) ([]int, string) {
	trace := func(shape []vhtypes.ShapePoint, match string) (*vhtypes.TraceAttributesResponse, error) {
		lookupCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return client.TraceAttributes(lookupCtx, region, &vhtypes.TraceAttributesRequest{
			Shape:      shape,
			Costing:    "auto",
			ShapeMatch: match,
			Filters: vhtypes.TraceFilters{
				Action:     "include",
				Attributes: roundaboutTraceAttributes,
			},
		})
	}

	// The exits up to the one taken, from the route itself.
	out, err := trace(points, "edge_walk")
	if err != nil {
		return nil, "route lookup failed: " + err.Error()
	}
	known, ok := logic.RoundaboutExitsFromEdges(out.Edges, exitCount)
	if !ok {
		return nil, "route lookup does not match the exit count"
	}

	// The rest of the lap, from a shape that carries on round the circle.
	more, ok := logic.ExtendAroundRing(ring)
	if !ok {
		return nil, "the roundabout is not round enough to walk"
	}
	lap := append(append(append([]vhtypes.ShapePoint{}, lead...), ring...), more...)
	full, err := trace(lap, "map_snap")
	if err != nil {
		return nil, "lap lookup failed: " + err.Error()
	}
	all, ok := logic.FullRoundaboutExits(full.Edges, known)
	if !ok {
		return nil, "lap lookup does not match the route lookup"
	}
	return all.Angles, ""
}

// roundaboutShape returns the shape points from [begin] to [end], with a few
// points around them, as lat/lon pairs. [shape] is flat [lat, lon, ...].
func roundaboutShape(shape []float64, begin, end int) []vhtypes.ShapePoint {
	n := len(shape) / 2
	from := max(0, begin-roundaboutShapeLead)
	to := min(n, end+roundaboutShapeTrail)
	if begin < 0 || end < begin || from >= to {
		return nil
	}
	points := make([]vhtypes.ShapePoint, 0, to-from)
	for i := from; i < to; i++ {
		points = append(points, vhtypes.ShapePoint{Lat: shape[2*i], Lon: shape[2*i+1]})
	}
	return points
}

// shapeSlice returns the shape points from [from] to [to], both included.
func shapeSlice(shape []float64, from, to int) []vhtypes.ShapePoint {
	n := len(shape) / 2
	if from < 0 || to < from || to >= n {
		return nil
	}
	points := make([]vhtypes.ShapePoint, 0, to-from+1)
	for i := from; i <= to; i++ {
		points = append(points, vhtypes.ShapePoint{Lat: shape[2*i], Lon: shape[2*i+1]})
	}
	return points
}

// shapeLead returns the few shape points before [begin].
func shapeLead(shape []float64, begin int) []vhtypes.ShapePoint {
	n := len(shape) / 2
	from := max(0, begin-roundaboutShapeLead)
	if begin <= from || begin > n {
		return nil
	}
	points := make([]vhtypes.ShapePoint, 0, begin-from)
	for i := from; i < begin; i++ {
		points = append(points, vhtypes.ShapePoint{Lat: shape[2*i], Lon: shape[2*i+1]})
	}
	return points
}

func int32s(values []int) []int32 {
	if len(values) == 0 {
		return nil
	}
	out := make([]int32, len(values))
	for i, v := range values {
		out[i] = int32(v)
	}
	return out
}
