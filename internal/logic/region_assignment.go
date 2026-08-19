package logic

import (
	"context"
	"math"
	"slices"

	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	log "github.com/swayrider/swlib/logger"
)

const (
	corridorWidthRatio = 0.2
	minCorridorWidthKm = 100.0
	maxCorridorWidthKm = 400.0
)

type RegionAssignment struct {
	Region string
	FromIndex int
	ToIndex int
	IsEmpty bool
}

func CalculateRegionAssignment(
	ctx context.Context,
	client regionQuerier,
	token string,
	locationList []*pbgeo.Coordinate,
	l *log.Logger,
) (
	assignmentList []*RegionAssignment,
	routePossible bool,
	err error,
) {
	lg := l.Derive(log.WithFunction("CalculateRegionAssignment"))

	resolveList, err := ResolveRegions(ctx, client, token, locationList, lg)
	if err != nil {
		lg.Errorf("Failed to resolve regions: %v", err)
		return
	}

	regionList := buildRegionList(resolveList)

	tmpAssignmentList := make([]*RegionAssignment, 0)
	var assignment *RegionAssignment
	for i := 0; i < len(regionList); i++ {
		if assignment == nil {
			assignment = &RegionAssignment{
				Region: regionList[i],
				FromIndex: i,
				ToIndex: i,
				IsEmpty: false,
			}
			continue
		}

		if regionList[i] == assignment.Region {
			assignment.ToIndex = i
			continue
		}

		tmpAssignmentList = append(tmpAssignmentList, assignment)
		assignment = &RegionAssignment{
			Region: regionList[i],
			FromIndex: i,
			ToIndex: i,
			IsEmpty: false,
		}
	}
	if assignment != nil {
		tmpAssignmentList = append(tmpAssignmentList, assignment)
	}

	waypoints := make([]regionclient.Coordinate, 0, len(locationList))
	for _, loc := range locationList {
		waypoints = append(waypoints, regionclient.Coordinate{Latitude: loc.Lat, Longitude: loc.Lon})
	}
	corridorPaths, corridorErr := client.FindRouteRegionPaths(
		ctx, token, waypoints, corridorWidth(locationList))
	if corridorErr != nil {
		lg.Warnf("FindRouteRegionPaths failed, falling back to FindRegionPath: %v", corridorErr)
	}
	var corridorPath []string
	for _, p := range corridorPaths {
		if corridorPath == nil || len(p) < len(corridorPath) {
			corridorPath = p
		}
	}

	assignmentList, routePossible, err = injectTransferRegions(
		ctx, client, token, tmpAssignmentList, corridorPath, lg)
	if err != nil {
		lg.Errorf("Failed to inject transfer regions: %v", err)
		return
	}

	return
}

func injectTransferRegions(
	ctx context.Context,
	client regionQuerier,
	token string,
	assignmentList []*RegionAssignment,
	corridorPath []string,
	l *log.Logger,
) (
	finalizedList []*RegionAssignment,
	possible bool,
	err error,
) {
	lg := l.Derive(log.WithFunction("injectTransferRegions"))

	finalizedList = make([]*RegionAssignment, 0, len(assignmentList)*2)
	finalizedList = append(finalizedList, assignmentList[0])
	possible = true
	for i := 1; i < len(assignmentList); i++ {
		fromRegion := assignmentList[i-1].Region
		toRegion := assignmentList[i].Region

		var path []string
		if sub := corridorSubPath(corridorPath, fromRegion, toRegion); sub != nil {
			path = sub
		} else {
			path, err = client.FindRegionPath(ctx, token, fromRegion, toRegion)
			if err != nil {
				lg.Errorf("Failed to find path between %s and %s: %v", fromRegion, toRegion, err)
				return
			}
		}

		if len(path) == 0 {
			possible = false
			return
		}

		if len(path) > 2 {
			for j := 1; j < len(path)-1; j++ {
				finalizedList = append(finalizedList, &RegionAssignment{
					Region:    path[j],
					FromIndex: -1,
					ToIndex:   -1,
					IsEmpty:   true,
				})
			}
		}
		finalizedList = append(finalizedList, assignmentList[i])
	}

	return
}

// corridorSubPath extracts the sub-path between fromRegion and toRegion from corridorPath.
// Returns nil if either region is absent or fromIdx >= toIdx.
func corridorSubPath(corridorPath []string, fromRegion, toRegion string) []string {
	fromIdx := slices.Index(corridorPath, fromRegion)
	toIdx := slices.Index(corridorPath, toRegion)
	if fromIdx < 0 || toIdx < 0 || fromIdx >= toIdx {
		return nil
	}
	return corridorPath[fromIdx : toIdx+1]
}

// corridorWidth computes the corridor width in km as a fraction of total path length,
// clamped to [minCorridorWidthKm, maxCorridorWidthKm].
func corridorWidth(locations []*pbgeo.Coordinate) float64 {
	var total float64
	for i := 1; i < len(locations); i++ {
		total += haversineKm(locations[i-1], locations[i])
	}
	return max(minCorridorWidthKm, min(maxCorridorWidthKm, total*corridorWidthRatio))
}

func haversineKm(a, b *pbgeo.Coordinate) float64 {
	const r = 6371.0
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)
	h := sinLat*sinLat + math.Cos(a.Lat*math.Pi/180)*math.Cos(b.Lat*math.Pi/180)*sinLon*sinLon
	return 2 * r * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

type regionCandidate struct {
	CoreRegion string
	ExtendsIntoRegion string
}

func buildRegionList(
	resolveList []*RegionResolvment,
) (
	regionList []string,
) {
	firstCore := resolveList[0].CoreRegions[0]
	lastCore := resolveList[len(resolveList)-1].CoreRegions[0]
	allInFirstCore := true
	allInLastCore := true

	candList := make([]*regionCandidate, 0)
	for i := 1; i < len(resolveList); i++ {
		rc := matchRegions(resolveList[i-1], resolveList[i])

		if i == 1 {
			// First loop we also need to set the first element
			if rc == nil {
				candList = append(candList, &regionCandidate{
					CoreRegion: resolveList[i-1].CoreRegions[0],
					ExtendsIntoRegion: "",
				})
			} else {
				candList = append(candList, rc)
			}
			if !resolveList[i-1].Contains(lastCore) {
				allInLastCore = false
			}
		}

		if rc == nil {
			candList = append(candList, &regionCandidate{
				CoreRegion: resolveList[i].CoreRegions[0],
				ExtendsIntoRegion: "",
			})
		} else {
			candList = append(candList, rc)
		}
		if !resolveList[i].Contains(firstCore) {
			allInFirstCore = false
		}
		if !resolveList[i].Contains(lastCore) {
			allInLastCore = false
		}
	}

	if allInFirstCore {
		regionList = make([]string, len(candList))
		for i := 0; i < len(candList); i++ {
			regionList[i] = firstCore
		}
		return
	}

	if allInLastCore {
		regionList = make([]string, len(candList))
		for i := 0; i < len(candList); i++ {
			regionList[i] = lastCore
		}
		return
	}

	regionList = resolveCandList(candList)
	return
}

func resolveCandList(
	candList []*regionCandidate,
) (
	regionList []string,
) {
	lastRegion := ""
	lst1 := make([]string, 0, len(candList))
	lst2 := make([]string, 0, len(candList))

	// Forward loop
	for _, rc := range candList {
		if lastRegion == "" {
			lastRegion = rc.CoreRegion
			lst1 = append(lst1, rc.CoreRegion)
			lst2 = append(lst2, "")
			continue
		}

		if rc.CoreRegion == lastRegion {
			lst1 = append(lst1, lastRegion)
			lst2 = append(lst2, "")
			continue
		}	

		if rc.ExtendsIntoRegion == lastRegion {
			lst1 = append(lst1, lastRegion)
			lst2 = append(lst2, rc.CoreRegion)
			continue
		}

		lst1 = append(lst1, rc.CoreRegion)
		lst2 = append(lst2, "")
		lastRegion = rc.CoreRegion
	}

	// Backward loop
	lastRegion = ""
	regionList = make([]string, len(candList))
	for i := len(lst1) - 1; i >= 0; i-- {
		if lastRegion == "" {
			regionList[i] = lst1[i]
			lastRegion = lst1[i]
			continue
		}

		if lst1[i] == lastRegion {
			regionList[i] = lst1[i]
			continue
		}

		if lst2[i] == lastRegion {
			regionList[i] = lst2[i]
			continue
		}
		regionList[i] = lst1[i]
		lastRegion = lst1[i]
	}
	return
}

func matchRegions(
	a, b *RegionResolvment,
) (
	res *regionCandidate,
){
	// CORE == CORE
	for _, cb := range b.CoreRegions {
		if slices.Contains(a.CoreRegions, cb) {
			res = &regionCandidate{
				CoreRegion: cb,
				ExtendsIntoRegion: "",
			}
			return
		}
	}

	// CORE == EXTENDED
	for _, cb := range b.CoreRegions {
		if slices.Contains(a.ExtendedRegions, cb) {
			res = &regionCandidate{
				CoreRegion: cb,
				ExtendsIntoRegion: "",
			}

			// EXTENDED == CORE
			for _, eb := range b.ExtendedRegions {
				if slices.Contains(a.CoreRegions, eb) {
					res.ExtendsIntoRegion = eb
				}
			}
			return
		}
	}

	// EXTENDED == CORE
	for _, eb := range b.ExtendedRegions {
		if slices.Contains(a.CoreRegions, eb) {
			res = &regionCandidate{
				CoreRegion: b.CoreRegions[0],
				ExtendsIntoRegion: eb,
			}
			return
		}
	}

	// EXTENDED == EXTENDED
	for _, eb := range b.ExtendedRegions {
		if slices.Contains(a.ExtendedRegions, eb) {
			res = &regionCandidate{
				CoreRegion: "",
				ExtendsIntoRegion: eb,
			}
			return
		}
	}

	return
}
