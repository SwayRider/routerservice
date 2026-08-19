package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	log "github.com/swayrider/swlib/logger"
)

// fakeRegionQuerier is a hand-written test double for regionQuerier. Each
// method delegates to its func field if set, otherwise returns a safe zero
// value, so tests only need to override the calls they actually care about.
type fakeRegionQuerier struct {
	searchPointFn func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error)

	findCrossingLocationsFn func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error)

	findRegionPathFn func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error)

	findRouteRegionPathsFn func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error)
}

func (f *fakeRegionQuerier) SearchPoint(
	ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool,
) (regionclient.RegionList, error) {
	if f.searchPointFn != nil {
		return f.searchPointFn(ctx, token, location, includeExtended)
	}
	return regionclient.RegionList{}, nil
}

func (f *fakeRegionQuerier) FindCrossingLocations(
	ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate,
	config regionclient.BorderCrossingConfig, limit int,
) ([]regionclient.BorderCrossing, error) {
	if f.findCrossingLocationsFn != nil {
		return f.findCrossingLocationsFn(ctx, token, fromRegion, toRegion, from, to, config, limit)
	}
	return nil, nil
}

func (f *fakeRegionQuerier) FindRegionPath(
	ctx context.Context, token, fromRegion, toRegion string,
) ([]string, error) {
	if f.findRegionPathFn != nil {
		return f.findRegionPathFn(ctx, token, fromRegion, toRegion)
	}
	return nil, nil
}

func (f *fakeRegionQuerier) FindRouteRegionPaths(
	ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64,
) ([][]string, error) {
	if f.findRouteRegionPathsFn != nil {
		return f.findRouteRegionPathsFn(ctx, token, waypoints, widthKm)
	}
	return nil, nil
}

// TestResolveRegion_Success verifies a happy-path SearchPoint result maps to
// a RegionResolvment with matching core/extended regions.
func TestResolveRegion_Success(t *testing.T) {
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			return regionclient.RegionList{CoreRegions: []string{"nl"}, ExtendedRegions: []string{"be"}}, nil
		},
	}

	got, err := ResolveRegion(context.Background(), fake, "tok", &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}, log.New())
	if err != nil {
		t.Fatalf("ResolveRegion error: %v", err)
	}
	if len(got.CoreRegions) != 1 || got.CoreRegions[0] != "nl" {
		t.Errorf("CoreRegions: want [nl], got %v", got.CoreRegions)
	}
	if len(got.ExtendedRegions) != 1 || got.ExtendedRegions[0] != "be" {
		t.Errorf("ExtendedRegions: want [be], got %v", got.ExtendedRegions)
	}
}

// TestResolveRegion_NoCoreRegions verifies the legal empty-without-error case
// (a location outside all known regions) maps to ErrLocationOutsideOfKnownRegions.
func TestResolveRegion_NoCoreRegions(t *testing.T) {
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			return regionclient.RegionList{}, nil
		},
	}

	got, err := ResolveRegion(context.Background(), fake, "tok", &pbgeo.Coordinate{Lat: 0, Lon: 0}, log.New())
	if got != nil {
		t.Errorf("want nil result, got %v", got)
	}
	if !errors.Is(err, ErrLocationOutsideOfKnownRegions) {
		t.Errorf("want ErrLocationOutsideOfKnownRegions, got %v", err)
	}
}

// TestResolveRegion_SearchPointError verifies a downstream error is propagated unwrapped.
func TestResolveRegion_SearchPointError(t *testing.T) {
	wantErr := errors.New("connection refused")
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			return regionclient.RegionList{}, wantErr
		},
	}

	got, err := ResolveRegion(context.Background(), fake, "tok", &pbgeo.Coordinate{Lat: 0, Lon: 0}, log.New())
	if got != nil {
		t.Errorf("want nil result, got %v", got)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

// TestResolveRegions_Success verifies each location is resolved in order.
func TestResolveRegions_Success(t *testing.T) {
	locations := []*pbgeo.Coordinate{
		{Lat: 52.3, Lon: 4.9},
		{Lat: 48.8, Lon: 2.3},
	}
	var seen []regionclient.Coordinate
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			seen = append(seen, location)
			if location.Latitude > 50 {
				return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
			}
			return regionclient.RegionList{CoreRegions: []string{"fr"}}, nil
		},
	}

	got, err := ResolveRegions(context.Background(), fake, "tok", locations, log.New())
	if err != nil {
		t.Fatalf("ResolveRegions error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 resolvments, got %d", len(got))
	}
	if got[0].CoreRegions[0] != "nl" || got[1].CoreRegions[0] != "fr" {
		t.Errorf("want [nl fr] in order, got [%v %v]", got[0].CoreRegions, got[1].CoreRegions)
	}
	if len(seen) != 2 || seen[0].Latitude != 52.3 || seen[1].Latitude != 48.8 {
		t.Errorf("SearchPoint called out of order or wrong count: %v", seen)
	}
}

// TestResolveRegions_PropagatesFirstError verifies the loop stops and returns
// immediately on the first failing location, matching the early-return in ResolveRegions.
func TestResolveRegions_PropagatesFirstError(t *testing.T) {
	locations := []*pbgeo.Coordinate{
		{Lat: 52.3, Lon: 4.9},
		{Lat: 48.8, Lon: 2.3},
		{Lat: 45.0, Lon: 1.0},
	}
	wantErr := errors.New("resolve failed")
	calls := 0
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			calls++
			if calls == 2 {
				return regionclient.RegionList{}, wantErr
			}
			return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
		},
	}

	got, err := ResolveRegions(context.Background(), fake, "tok", locations, log.New())
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
	if len(got) != 1 {
		t.Errorf("want partial resolveList of len 1 (only the successful call before the error), got %d", len(got))
	}
	if calls != 2 {
		t.Errorf("want exactly 2 SearchPoint calls (stop on error), got %d", calls)
	}
}

// TestResolveRegions_EmptyInput verifies no client calls are made and an
// empty, non-nil list is returned for an empty input.
func TestResolveRegions_EmptyInput(t *testing.T) {
	called := false
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			called = true
			return regionclient.RegionList{}, nil
		},
	}

	got, err := ResolveRegions(context.Background(), fake, "tok", nil, log.New())
	if err != nil {
		t.Fatalf("ResolveRegions error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty resolveList, got %v", got)
	}
	if called {
		t.Error("SearchPoint should not be called for empty input")
	}
}
