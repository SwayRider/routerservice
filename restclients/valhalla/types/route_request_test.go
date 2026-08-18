package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRouteRequest_ExcludeFieldsWireNames locks in the Valhalla wire field
// names for ExcludeLocations/ExcludePolygons. Valhalla's current API uses
// exclude_locations/exclude_polygons at the top level of the request — not
// the older avoid_locations/avoid_polygons phrasing that appears only in
// some prose in Valhalla's own docs.
func TestRouteRequest_ExcludeFieldsWireNames(t *testing.T) {
	req := &RouteRequest{
		Costing:        Motorcycle,
		CostingOptions: CostingOptions{},
		Locations: []Location{
			{Lat: 50.0, Lon: 4.0},
			{Lat: 51.0, Lon: 5.0},
		},
		ExcludeLocations: []Location{
			{Lat: 50.5, Lon: 4.5},
		},
		ExcludePolygons: [][][]float64{
			{{4.0, 51.0}, {4.1, 51.1}, {4.2, 51.2}},
		},
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	body := string(b)

	if !strings.Contains(body, `"exclude_locations"`) {
		t.Errorf("want %q in marshaled JSON, got: %s", "exclude_locations", body)
	}
	if !strings.Contains(body, `"exclude_polygons"`) {
		t.Errorf("want %q in marshaled JSON, got: %s", "exclude_polygons", body)
	}
	if strings.Contains(body, `"avoid_locations"`) {
		t.Errorf("wire JSON must not use the deprecated %q key, got: %s", "avoid_locations", body)
	}
	if strings.Contains(body, `"avoid_polygons"`) {
		t.Errorf("wire JSON must not use the deprecated %q key, got: %s", "avoid_polygons", body)
	}
}

func TestRouteRequest_ExcludeFieldsOmittedWhenEmpty(t *testing.T) {
	req := &RouteRequest{
		Costing:        Motorcycle,
		CostingOptions: CostingOptions{},
		Locations: []Location{
			{Lat: 50.0, Lon: 4.0},
			{Lat: 51.0, Lon: 5.0},
		},
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	body := string(b)

	if strings.Contains(body, "exclude_locations") {
		t.Errorf("want exclude_locations omitted when empty, got: %s", body)
	}
	if strings.Contains(body, "exclude_polygons") {
		t.Errorf("want exclude_polygons omitted when empty, got: %s", body)
	}
}
