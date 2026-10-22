package server

import (
	"slices"
	"testing"

	"github.com/swayrider/swlib/security"
)

// Every RPC needs an endpoint profile, otherwise the auth interceptor answers
// "unauthenticated" for it (Rejoin was missing once).
func TestRouterEndpointsAreRegistered(t *testing.T) {
	for _, endpoint := range []string{
		"/router.v1.RouterService/Route",
		"/router.v1.RouterService/Rejoin",
	} {
		profile := security.GetEndpointProfile(endpoint)
		if !profile.AllowUser || !profile.AllowService {
			t.Errorf("%s: want users and services allowed, got %+v", endpoint, profile)
		}
		if !slices.Contains(profile.AllowedScopes, "routing:execute") {
			t.Errorf("%s: want scope routing:execute, got %v", endpoint, profile.AllowedScopes)
		}
	}
}
