package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	healthv1 "github.com/swayrider/protos/health/v1"
	"github.com/swayrider/routerservice/internal/valhalla"
	log "github.com/swayrider/swlib/logger"
)

type fakeRegionPinger struct {
	err error
}

func (f fakeRegionPinger) Ping() error {
	return f.err
}

func testValhallaConfig(t *testing.T, srv *httptest.Server) *valhalla.Config {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("failed to parse test server port: %v", err)
	}
	return &valhalla.Config{
		ValhallaHosts: map[string]string{"be": u.Hostname()},
		ValhallaPorts: map[string]int{"be": port},
	}
}

func TestCheck_AllDependenciesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewHealthServer(fakeRegionPinger{}, testValhallaConfig(t, srv), log.New())

	resp, err := h.Check(context.Background(), &healthv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}
	if resp.Status != healthv1.HealthResponse_UP {
		t.Errorf("Check().Status = %v, want UP", resp.Status)
	}
}

func TestCheck_RegionserviceDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewHealthServer(fakeRegionPinger{err: errors.New("connection refused")}, testValhallaConfig(t, srv), log.New())

	resp, err := h.Check(context.Background(), &healthv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}
	if resp.Status != healthv1.HealthResponse_DOWN {
		t.Errorf("Check().Status = %v, want DOWN", resp.Status)
	}
}

func TestCheck_ValhallaDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := NewHealthServer(fakeRegionPinger{}, testValhallaConfig(t, srv), log.New())

	resp, err := h.Check(context.Background(), &healthv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}
	if resp.Status != healthv1.HealthResponse_DOWN {
		t.Errorf("Check().Status = %v, want DOWN", resp.Status)
	}
}

func TestCheck_NoValhallaHostsConfigured(t *testing.T) {
	h := NewHealthServer(fakeRegionPinger{}, &valhalla.Config{}, log.New())

	resp, err := h.Check(context.Background(), &healthv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}
	if resp.Status != healthv1.HealthResponse_UP {
		t.Errorf("Check().Status = %v, want UP", resp.Status)
	}
}
