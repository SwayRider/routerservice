package logic

import (
	"slices"

	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	log "github.com/swayrider/swlib/logger"
)

type RegionAssignment struct {
	Region string
	FromIndex int
	ToIndex int
	IsEmpty bool
}

func CalculateRegionAssignment(
	client *regionclient.Client,
	locationList []*pbgeo.Coordinate,
	l *log.Logger,
) (
	assignmentList []*RegionAssignment,
	routePossible bool,
	err error,
) {
	lg := l.Derive(log.WithFunction("CalculateRegionAssignment"))

	resolveList, err := ResolveRegions(client, locationList, lg)
	if err != nil {
		lg.Errorf("Failed to resolve regions: %v", err)
		err = ErrNoRouteFound
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

	assignmentList, routePossible, err  = injectTransferRegions(
		client, tmpAssignmentList, lg)
	if err != nil {
		lg.Errorf("Failed to inject transfer regions: %v", err)
		return
	}

	return
}

func injectTransferRegions(
	client *regionclient.Client,
	assignmentList []*RegionAssignment,
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
		path, err = client.FindRegionPath(fromRegion, toRegion)
		if err != nil {
			lg.Errorf("Failed to find path between %s and %s: %v", fromRegion, toRegion, err)
			return
		}

		if len(path) == 0 {
			possible = false
			return
		}

		if len(path) > 2 {
			for j := 1; j < len(path)-1; j++ {
				finalizedList = append(finalizedList, &RegionAssignment{
					Region: path[j],
					FromIndex: -1,
					ToIndex: -1,
					IsEmpty: true,
				})
			}
		}
		finalizedList = append(finalizedList, assignmentList[i])
	}

	return
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

	regionList =  resolveCandList(candList)
	return

	/*for i := 1; i < len(resolveList); i++ {
		if rc == nil {
			if i == 1 {
				candList = append(candList, &regionCandidate{
					CoreRegion: resolveList[i-1].CoreRegions[0],
					ExtendsIntoRegion: "",
				})
			}
			candList = append(candList, &regionCandidate{
				CoreRegion: resolveList[i].CoreRegions[0],
				ExtendsIntoRegion: "",
			})
			if !slices.Contains(resolveList[i].CoreRegions, firstCore) && !slices.Contains(resolveList[i].ExtendedRegions, firstCore) {
				allInFirstCore = false
			}
			if !slices.Contains(resolveList[i].CoreRegions, lastCore) && !slices.Contains(resolveList[i].ExtendedRegions, lastCore) {
				allInLastCore = false
			}
			continue
		}
		if i == 1 {
			candList = append(candList, rc)
		}
		candList = append(candList, rc)
		if !slices.Contains(resolveList[i].CoreRegions, firstCore) && !slices.Contains(resolveList[i].ExtendedRegions, firstCore) {
			allInFirstCore = false
		}
		if !slices.Contains(resolveList[i].CoreRegions, lastCore) && !slices.Contains(resolveList[i].ExtendedRegions, lastCore) {
			allInLastCore = false
		}
	}*/

	/*lastRegion := ""
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
	}*/

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
