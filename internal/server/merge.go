package server

import (
	"fmt"
	"math"

	"github.com/twpayne/go-polyline"
	"google.golang.org/protobuf/proto"

	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
)

var polylineCodec = polyline.Codec{Dim: 2, Scale: 1e6}

// borderOverlapToleranceMeters is the max distance between two legs' shared
// border point below which they're treated as the same point. Independent
// Valhalla instances snap to their own regional road graphs, so the same
// real-world border point routinely differs by more than a trivial amount
// between legs.
const borderOverlapToleranceMeters = 5.0

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)
	h := sinLat*sinLat + math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*sinLon*sinLon
	return 2 * r * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

// mergeRouteResponse merges legs connected by MergeNext into single legs,
// producing a response structurally identical to a single Valhalla response.
func mergeRouteResponse(resp *routerv1.RouteResponse) error {
	if resp == nil || resp.Trip == nil {
		return nil
	}

	trip := resp.Trip
	legs := trip.Legs
	if len(legs) < 2 {
		return nil
	}

	groups := findMergeGroups(legs)
	if len(groups) == 0 {
		return nil
	}

	for g := len(groups) - 1; g >= 0; g-- {
		grp := groups[g]
		merged, err := mergeLegGroup(legs[grp.start : grp.end+1])
		if err != nil {
			return fmt.Errorf("merge leg group [%d,%d]: %w", grp.start, grp.end, err)
		}

		newLegs := make([]*routerv1.Leg, 0, len(legs)-(grp.end-grp.start))
		newLegs = append(newLegs, legs[:grp.start]...)
		newLegs = append(newLegs, merged)
		newLegs = append(newLegs, legs[grp.end+1:]...)
		trip.Legs = newLegs
		legs = newLegs
	}

	return nil
}

type mergeGroup struct {
	start, end int
}

func findMergeGroups(legs []*routerv1.Leg) []mergeGroup {
	var groups []mergeGroup
	for i := 0; i < len(legs); i++ {
		if !legs[i].MergeNext {
			continue
		}
		start := i
		for i < len(legs) && legs[i].MergeNext {
			i++
		}
		if i < len(legs) {
			groups = append(groups, mergeGroup{start: start, end: i})
		} else {
			groups = append(groups, mergeGroup{start: start, end: start})
		}
	}
	return groups
}

func mergeLegGroup(legs []*routerv1.Leg) (*routerv1.Leg, error) {
	if len(legs) == 0 {
		return nil, fmt.Errorf("empty leg group")
	}
	if len(legs) == 1 {
		legs[0].MergeNext = false
		return legs[0], nil
	}

	shapes := make([]string, len(legs))
	hasElevation := true
	for i, leg := range legs {
		shapes[i] = leg.Shape
		if leg.Elevation == nil {
			hasElevation = false
		}
	}

	mergedShape, offsets, mergedElevation, mergedElevInterval, err := mergeShapes(shapes, legs, hasElevation)
	if err != nil {
		return nil, fmt.Errorf("merge shapes: %w", err)
	}

	mergedManeuvers := flattenManeuvers(legs, offsets)

	mergedManeuvers = handleBorderManeuvers(mergedManeuvers, legs)

	summaries := make([]*routerv1.Summary, len(legs))
	for i, leg := range legs {
		summaries[i] = leg.Summary
	}
	mergedSummary := mergeSummaries(summaries)

	merged := &routerv1.Leg{
		Shape:             mergedShape,
		Maneuvers:         mergedManeuvers,
		Summary:           mergedSummary,
		Elevation:         mergedElevation,
		ElevationInterval: mergedElevInterval,
		MergeNext:         false,
	}
	return merged, nil
}

func mergeShapes(shapes []string, legs []*routerv1.Leg, hasElevation bool) (
	mergedShape string,
	offsets []int,
	mergedElevation []float64,
	elevationInterval *float64,
	err error,
) {
	offsets = make([]int, len(shapes))
	var flat []float64

	if hasElevation {
		mergedElevation = make([]float64, 0)
		elevationInterval = legs[0].ElevationInterval
	}

	for i, shape := range shapes {
		points, _, decErr := polylineCodec.DecodeFlatCoords(nil, []byte(shape))
		if decErr != nil {
			err = fmt.Errorf("decode shape[%d]: %w", i, decErr)
			return
		}
		if len(points)%2 != 0 {
			err = fmt.Errorf("shape[%d]: odd number of coordinates", i)
			return
		}

		overlap := 0
		if i > 0 && len(flat) >= 2 && len(points) >= 2 {
			prevLat, prevLon := flat[len(flat)-2], flat[len(flat)-1]
			curLat, curLon := points[0], points[1]
			if haversineMeters(prevLat, prevLon, curLat, curLon) < borderOverlapToleranceMeters {
				overlap = 2
			}
		}

		offsets[i] = len(flat) / 2
		if overlap > 0 {
			offsets[i]--
		}
		flat = append(flat, points[overlap:]...)

		if hasElevation && i > 0 && legs[i].Elevation != nil {
			skip := 0
			if overlap > 0 {
				skip = 1
			}
			mergedElevation = append(mergedElevation, legs[i].Elevation[skip:]...)
		} else if hasElevation && i == 0 && legs[i].Elevation != nil {
			mergedElevation = append(mergedElevation, legs[i].Elevation...)
		}
	}

	encoded, encErr := polylineCodec.EncodeFlatCoords(nil, flat)
	if encErr != nil {
		err = fmt.Errorf("encode merged shape: %w", encErr)
		return
	}
	mergedShape = string(encoded)
	return
}

func flattenManeuvers(legs []*routerv1.Leg, offsets []int) []*routerv1.Maneuver {
	var result []*routerv1.Maneuver
	for i, leg := range legs {
		for _, m := range leg.Maneuvers {
			clone := proto.Clone(m).(*routerv1.Maneuver)
			clone.BeginShapeIndex += int32(offsets[i])
			clone.EndShapeIndex += int32(offsets[i])
			result = append(result, clone)
		}
	}
	return result
}

func handleBorderManeuvers(maneuvers []*routerv1.Maneuver, legs []*routerv1.Leg) []*routerv1.Maneuver {
	legBdry := make([]int, len(legs)+1)
	idx := 0
	for i, leg := range legs {
		legBdry[i] = idx
		idx += len(leg.Maneuvers)
	}
	legBdry[len(legs)] = idx

	for legIdx := 0; legIdx < len(legs)-1; legIdx++ {
		lastOfLeg := legBdry[legIdx+1] - 1
		firstOfNext := legBdry[legIdx+1]
		_ = firstOfNext

		if lastOfLeg < 0 || firstOfNext >= len(maneuvers) {
			continue
		}

		lastM := maneuvers[lastOfLeg]
		firstM := maneuvers[firstOfNext]

		if lastM.Type != routerv1.ManeuverType_M_DESTINATION &&
			lastM.Type != routerv1.ManeuverType_M_DESTINATION_RIGHT &&
			lastM.Type != routerv1.ManeuverType_M_DESTINATION_LEFT {
			lastM.Type = routerv1.ManeuverType_M_DESTINATION
		}

		prevEndShape := lastM.BeginShapeIndex
		if lastOfLeg > 0 {
			prevEndShape = maneuvers[lastOfLeg-1].EndShapeIndex
		}

		nextBeginShape := firstM.EndShapeIndex
		if firstOfNext+1 < len(maneuvers) {
			nextBeginShape = maneuvers[firstOfNext+1].BeginShapeIndex
		}

		continueM := &routerv1.Maneuver{
			Type:          routerv1.ManeuverType_M_CONTINUE,
			Instruction:   "Continue straight.",
			Time:          lastM.Time + firstM.Time,
			Length:        lastM.Length + firstM.Length,
			BeginShapeIndex: prevEndShape,
			EndShapeIndex:   nextBeginShape,
			TravelMode:    lastM.TravelMode,
			TravelType:    lastM.TravelType,
			BearingBefore: lastM.BearingBefore,
			BearingAfter:  firstM.BearingAfter,
		}
		if continueM.BearingBefore == 0 && firstM.BearingBefore != 0 {
			continueM.BearingBefore = firstM.BearingBefore
		}
		if continueM.BearingAfter == 0 && lastM.BearingAfter != 0 {
			continueM.BearingAfter = lastM.BearingAfter
		}

		maneuvers[lastOfLeg] = continueM
		maneuvers = append(maneuvers[:firstOfNext], maneuvers[firstOfNext+1:]...)

		for j := legIdx + 1; j < len(legBdry); j++ {
			legBdry[j]--
		}
	}

	return maneuvers
}

func mergeSummaries(summaries []*routerv1.Summary) *routerv1.Summary {
	s := &routerv1.Summary{}
	for _, summary := range summaries {
		if summary == nil {
			continue
		}
		s.Time += summary.Time
		s.Length += summary.Length
		if summary.HasToll {
			s.HasToll = true
		}
		if summary.HasHighway {
			s.HasHighway = true
		}
		if summary.HasFerry {
			s.HasFerry = true
		}
		if summary.BoundingBox != nil {
			if s.BoundingBox == nil {
				s.BoundingBox = &pbgeo.BoundingBox{
					BottomLeft: &pbgeo.Coordinate{
						Lat: summary.BoundingBox.BottomLeft.Lat,
						Lon: summary.BoundingBox.BottomLeft.Lon,
					},
					TopRight: &pbgeo.Coordinate{
						Lat: summary.BoundingBox.TopRight.Lat,
						Lon: summary.BoundingBox.TopRight.Lon,
					},
				}
			} else {
				if summary.BoundingBox.BottomLeft.Lat < s.BoundingBox.BottomLeft.Lat {
					s.BoundingBox.BottomLeft.Lat = summary.BoundingBox.BottomLeft.Lat
				}
				if summary.BoundingBox.BottomLeft.Lon < s.BoundingBox.BottomLeft.Lon {
					s.BoundingBox.BottomLeft.Lon = summary.BoundingBox.BottomLeft.Lon
				}
				if summary.BoundingBox.TopRight.Lat > s.BoundingBox.TopRight.Lat {
					s.BoundingBox.TopRight.Lat = summary.BoundingBox.TopRight.Lat
				}
				if summary.BoundingBox.TopRight.Lon > s.BoundingBox.TopRight.Lon {
					s.BoundingBox.TopRight.Lon = summary.BoundingBox.TopRight.Lon
				}
			}
		}
	}
	return s
}

// removeBorderLocations removes region crossing locations recorded during
// the stitching loop. borderIdx contains the pre-stitch index of each
// region crossing in the original locations array.
func removeBorderLocations(trip *routerv1.Trip, borderIdx map[int]bool) {
	if len(borderIdx) == 0 {
		return
	}
	filtered := make([]*routerv1.RouteLocationReturned, 0, len(trip.Locations))
	for i, loc := range trip.Locations {
		if !borderIdx[i] {
			filtered = append(filtered, loc)
		}
	}
	trip.Locations = filtered
}
