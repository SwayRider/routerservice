package valhalla_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/swayrider/routerservice/restclients/valhalla"
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
