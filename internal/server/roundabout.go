package server

import (
	"context"
	"sync"
	"time"

	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	"github.com/swayrider/routerservice/internal/logic"
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

// addRoundaboutArms fills in the arms of every roundabout of [resp], which
// Valhalla only numbers (the exit to take). It is best effort: when a lookup
// fails, times out or does not agree with Valhalla's own exit count, that
// maneuver keeps no arms and the route is returned unchanged.
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
					lg.Debugf("roundabout arms: bad leg shape: %v", err)
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
			exitCount := *m.RoundaboutExitCount

			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				lookupCtx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()
				out, err := client.TraceAttributes(lookupCtx, region, &vhtypes.TraceAttributesRequest{
					Shape:      points,
					Costing:    "auto",
					ShapeMatch: "edge_walk",
					Filters: vhtypes.TraceFilters{
						Action:     "include",
						Attributes: roundaboutTraceAttributes,
					},
				})
				if err != nil {
					lg.Debugf("roundabout arms: lookup failed: %v", err)
					return
				}
				arms, ok := logic.RoundaboutArmsFromEdges(out.Edges, exitCount)
				if !ok {
					lg.Debugln("roundabout arms: lookup does not match the exit count, ignored")
					return
				}
				m.RoundaboutExitAngles = arms.ExitAngles
				m.RoundaboutEntryAngles = arms.EntryAngles
			}()
		}
	}
	wg.Wait()
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
