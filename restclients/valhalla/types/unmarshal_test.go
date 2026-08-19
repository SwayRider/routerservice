package types

import (
	"encoding/json"
	"testing"
)

// TestRouteResponse_Unmarshal verifies a full decode of a Valhalla /route
// response down to one representative leg and location.
func TestRouteResponse_Unmarshal(t *testing.T) {
	body := `{
		"id": "route-1",
		"trip": {
			"status": 0,
			"status_message": "Found route between points",
			"units": "km",
			"language": "en-US",
			"locations": [{"lat": 50.0, "lon": 4.0}],
			"legs": [{
				"maneuvers": [],
				"shape": "abc",
				"summary": {"time": 100, "length": 10, "has_toll": true, "has_highway": false, "has_ferry": false, "min_lat": 50, "min_lon": 4, "max_lat": 51, "max_lon": 5}
			}],
			"summary": {"time": 100, "length": 10, "has_toll": true, "has_highway": false, "has_ferry": false, "min_lat": 50, "min_lon": 4, "max_lat": 51, "max_lon": 5}
		}
	}`

	var got RouteResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if got.Id == nil || *got.Id != "route-1" {
		t.Errorf("Id: want route-1, got %v", got.Id)
	}
	if got.Trip.Status != 0 || got.Trip.StatusMessage != "Found route between points" {
		t.Errorf("Trip status: unexpected value %+v", got.Trip)
	}
	if got.Trip.Units != Kilometers || got.Trip.Language != Language("en-US") {
		t.Errorf("Trip units/language: unexpected value %v/%v", got.Trip.Units, got.Trip.Language)
	}
	if len(got.Trip.Locations) != 1 || got.Trip.Locations[0].Lat != 50.0 {
		t.Errorf("Trip locations: unexpected value %+v", got.Trip.Locations)
	}
	if len(got.Trip.Legs) != 1 || got.Trip.Legs[0].Shape != "abc" {
		t.Errorf("Trip legs: unexpected value %+v", got.Trip.Legs)
	}
	if !got.Trip.Summary.HasToll || got.Trip.Summary.MaxLat != 51 {
		t.Errorf("Trip summary: unexpected value %+v", got.Trip.Summary)
	}
}

// TestRouteResponse_Unmarshal_OmittedId verifies a response with no "id" key
// decodes to a nil Id.
func TestRouteResponse_Unmarshal_OmittedId(t *testing.T) {
	body := `{"trip":{"status":0,"status_message":"","units":"kilometers","language":"en-US","locations":[],"legs":[],"summary":{"time":0,"length":0,"has_toll":false,"has_highway":false,"has_ferry":false,"min_lat":0,"min_lon":0,"max_lat":0,"max_lon":0}}}`

	var got RouteResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if got.Id != nil {
		t.Errorf("Id: want nil, got %v", *got.Id)
	}
}

// TestTrip_Unmarshal_EmptyLegsAndLocations verifies empty legs/locations
// arrays decode cleanly with no error.
func TestTrip_Unmarshal_EmptyLegsAndLocations(t *testing.T) {
	body := `{"status":0,"status_message":"","units":"kilometers","language":"en-US","locations":[],"legs":[],"summary":{"time":0,"length":0,"has_toll":false,"has_highway":false,"has_ferry":false,"min_lat":0,"min_lon":0,"max_lat":0,"max_lon":0}}`

	var got Trip
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if len(got.Legs) != 0 || len(got.Locations) != 0 {
		t.Errorf("want empty legs/locations, got legs=%v locations=%v", got.Legs, got.Locations)
	}
}

// TestLocateRequest_Marshal guards NewLocateRequest's contract: verbose is
// forced true and locations wire shape matches Valhalla's expected format.
func TestLocateRequest_Marshal(t *testing.T) {
	req := NewLocateRequest(51.0, 4.0)

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if decoded["verbose"] != true {
		t.Errorf("verbose: want true, got %v", decoded["verbose"])
	}
	locs, ok := decoded["locations"].([]any)
	if !ok || len(locs) != 1 {
		t.Fatalf("locations: want 1 entry, got %v", decoded["locations"])
	}
	loc := locs[0].(map[string]any)
	if loc["lat"] != 51.0 || loc["lon"] != 4.0 {
		t.Errorf("locations[0]: want lat=51.0 lon=4.0, got %v", loc)
	}
}

// TestLocateResponse_Unmarshal_Full verifies a full decode including one
// edge with a populated "edge" details object and one with it omitted (the
// sparse case getRoadType guards against).
func TestLocateResponse_Unmarshal_Full(t *testing.T) {
	body := `{
		"input_lat": 51.0,
		"input_lon": 4.0,
		"nodes": [{"lat": 51.0, "lon": 4.0}],
		"edges": [
			{"edge": {"classification": {"classification": "motorway"}}},
			{}
		],
		"warnings": ["edge_distance"]
	}`

	var got LocateResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if got.InputLat != 51.0 || got.InputLon != 4.0 {
		t.Errorf("InputLat/InputLon: unexpected value %v/%v", got.InputLat, got.InputLon)
	}
	if len(got.Nodes) != 1 {
		t.Errorf("want 1 node, got %d", len(got.Nodes))
	}
	if len(got.Edges) != 2 {
		t.Fatalf("want 2 edges, got %d", len(got.Edges))
	}
	if got.Edges[0].Edge == nil || got.Edges[0].Edge.Classification.Classification != Motorway {
		t.Errorf("Edges[0].Edge: want populated with Motorway classification, got %v", got.Edges[0].Edge)
	}
	if got.Edges[1].Edge != nil {
		t.Errorf("Edges[1].Edge: want nil (sparse case), got %v", got.Edges[1].Edge)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "edge_distance" {
		t.Errorf("Warnings: unexpected value %v", got.Warnings)
	}
}

// TestLocateResponse_Unmarshal_Empty verifies an empty object decodes to all
// zero-value fields without error.
func TestLocateResponse_Unmarshal_Empty(t *testing.T) {
	var got LocateResponse
	if err := json.Unmarshal([]byte(`{}`), &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if got.InputLat != 0 || got.InputLon != 0 || got.Nodes != nil || got.Edges != nil || got.Warnings != nil {
		t.Errorf("want all zero-value fields, got %+v", got)
	}
}
