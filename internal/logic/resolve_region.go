package logic

import (
	"slices"
	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	log "github.com/swayrider/swlib/logger"
)


type RegionResolvment struct {
	CoreRegions []string
	ExtendedRegions []string
}

func (r RegionResolvment) Contains(region string) bool {
	return slices.Contains(r.CoreRegions, region) ||
		slices.Contains(r.ExtendedRegions, region)
}
	
func ResolveRegions(
	client *regionclient.Client,
	locations []*pbgeo.Coordinate,
	l *log.Logger,
) (
	resolveList []*RegionResolvment,
	err error,
) {
	lg := l.Derive(log.WithFunction("ResolveRegions"))

	resolveList = make([]*RegionResolvment, 0, len(locations))
	for _, location := range locations {
		var resolvment *RegionResolvment
		resolvment, err = ResolveRegion(client, location, lg)
		if err != nil {
			lg.Errorf("Region resolvment failed: %v", err)
			return
		}
		resolveList = append(resolveList, resolvment)
	}
	return
}

func ResolveRegion(
	client *regionclient.Client,
	location *pbgeo.Coordinate,
	l *log.Logger,
) (
	resolvment *RegionResolvment,
	err error,
) {
	lg := l.Derive(log.WithFunction("ResolveRegion"))

	coord := regionclient.Coordinate{
		Latitude:  location.Lat,
		Longitude: location.Lon,
	}

	regionList, err := client.SearchPoint(coord, true)
	if err != nil {
		lg.Errorf("Failed to resolve region for coordinate %v: %v", coord, err)
		return
	}

	if len(regionList.CoreRegions) == 0 {
		lg.Errorf("Failed to resolve region for coordinate %v: %v", coord, err)
		err = ErrLocationOutsideOfKnownRegions
		return
	}

	resolvment = &RegionResolvment{
		CoreRegions: regionList.CoreRegions,
		ExtendedRegions: regionList.ExtendedRegions,
	}
	return
}
