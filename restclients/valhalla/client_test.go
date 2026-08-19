package valhalla_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/swayrider/routerservice/restclients/valhalla"
	"github.com/swayrider/routerservice/restclients/valhalla/types"
)

func addTestRegion(t *testing.T, clnt *valhalla.Client, region string, srv *httptest.Server) {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("failed to parse test server port: %v", err)
	}
	clnt.AddRegion(region, u.Hostname(), port)
}

func TestStatus_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	if err := clnt.Status(context.Background(), "be"); err != nil {
		t.Errorf("Status() = %v, want nil", err)
	}
}

func TestStatus_NonOKResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	if err := clnt.Status(context.Background(), "be"); err == nil {
		t.Error("Status() = nil, want error for a 500 response")
	}
}

func TestStatus_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("failed to parse test server port: %v", err)
	}
	srv.Close() // closed before use so the port is unreachable

	clnt := valhalla.NewClient()
	clnt.AddRegion("be", u.Hostname(), port)

	if err := clnt.Status(context.Background(), "be"); err == nil {
		t.Error("Status() = nil, want error for an unreachable host")
	}
}

func TestStatus_UnknownRegion(t *testing.T) {
	clnt := valhalla.NewClient()
	if err := clnt.Status(context.Background(), "missing"); err == nil {
		t.Error("Status() = nil, want error for an unregistered region")
	}
}

func TestLocate_OK(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"input_lat":51.0,"input_lon":4.0,"warnings":["edge_distance"]}]`))
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	got, err := clnt.Locate(context.Background(), "be", types.NewLocateRequest(51.0, 4.0))
	if err != nil {
		t.Fatalf("Locate() error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/locate" {
		t.Errorf("want POST /locate, got %s %s", gotMethod, gotPath)
	}
	if got == nil {
		t.Fatal("want non-nil response")
	}
	if got.InputLat != 51.0 || got.InputLon != 4.0 {
		t.Errorf("InputLat/InputLon: want (51.0,4.0), got (%v,%v)", got.InputLat, got.InputLon)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "edge_distance" {
		t.Errorf("Warnings: unexpected value %v", got.Warnings)
	}
}

func TestLocate_EmptyArrayResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	got, err := clnt.Locate(context.Background(), "be", types.NewLocateRequest(51.0, 4.0))
	if err != nil {
		t.Fatalf("Locate() error: %v", err)
	}
	if got != nil {
		t.Errorf("want nil response for an empty array, got %v", got)
	}
}

func TestLocate_NonOKResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	_, err := clnt.Locate(context.Background(), "be", types.NewLocateRequest(51.0, 4.0))
	if err == nil {
		t.Error("Locate() = nil error, want error for a 500 response")
	}
}

func TestLocate_UnknownRegion(t *testing.T) {
	clnt := valhalla.NewClient()
	_, err := clnt.Locate(context.Background(), "missing", types.NewLocateRequest(51.0, 4.0))
	if err == nil {
		t.Error("Locate() = nil error, want error for an unregistered region")
	}
}

func TestLocate_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	_, err := clnt.Locate(context.Background(), "be", types.NewLocateRequest(51.0, 4.0))
	if err == nil {
		t.Error("Locate() = nil error, want a decode error for malformed JSON")
	}
}

func TestRoute_OK(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trip":{"status":0,"status_message":"Found route","units":"kilometers","language":"en-US","locations":[],"legs":[],"summary":{"time":100,"length":10,"has_toll":false,"has_highway":true,"has_ferry":false,"min_lat":50,"min_lon":4,"max_lat":51,"max_lon":5}}}`))
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	got, err := clnt.Route(context.Background(), "be", types.NewRouteRequest(types.Auto))
	if err != nil {
		t.Fatalf("Route() error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/route" {
		t.Errorf("want POST /route, got %s %s", gotMethod, gotPath)
	}
	if got.Id != nil {
		t.Errorf("Id: want nil (omitted in response), got %v", *got.Id)
	}
	if got.Trip.Status != 0 || got.Trip.StatusMessage != "Found route" {
		t.Errorf("Trip status: unexpected value %+v", got.Trip)
	}
	if !got.Trip.Summary.HasHighway || got.Trip.Summary.HasToll {
		t.Errorf("Trip summary flags: unexpected value %+v", got.Trip.Summary)
	}
}

func TestRoute_NonOKResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	_, err := clnt.Route(context.Background(), "be", types.NewRouteRequest(types.Auto))
	if err == nil {
		t.Error("Route() = nil error, want error for a 500 response")
	}
}

func TestRoute_UnknownRegion(t *testing.T) {
	clnt := valhalla.NewClient()
	_, err := clnt.Route(context.Background(), "missing", types.NewRouteRequest(types.Auto))
	if err == nil {
		t.Error("Route() = nil error, want error for an unregistered region")
	}
}

func TestRoute_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	clnt := valhalla.NewClient()
	addTestRegion(t, clnt, "be", srv)

	_, err := clnt.Route(context.Background(), "be", types.NewRouteRequest(types.Auto))
	if err == nil {
		t.Error("Route() = nil error, want a decode error for malformed JSON")
	}
}
