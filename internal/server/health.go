package server

import (
	"context"
	"time"

	healthv1 "github.com/swayrider/protos/health/v1"
	"github.com/swayrider/routerservice/internal/valhalla"
)

const dependencyCheckTimeout = 3 * time.Second

// Check reports UP only when every dependency routerservice relies on —
// regionservice, and each explicitly configured Valhalla instance — is reachable.
func (s *HealthServer) Check(
	ctx context.Context,
	req *healthv1.HealthRequest,
) (*healthv1.HealthResponse, error) {
	if err := s.regionClient.Ping(); err != nil {
		s.l.Debugf("regionservice dependency check failed: %v", err)
		return &healthv1.HealthResponse{Status: healthv1.HealthResponse_DOWN}, nil
	}

	regions := make([]string, 0, len(s.valhallaConfig.ValhallaHosts))
	for region := range s.valhallaConfig.ValhallaHosts {
		regions = append(regions, region)
	}
	vhClient := valhalla.GetClientForRegions(s.valhallaConfig, regions)

	for _, region := range regions {
		checkCtx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
		err := vhClient.Status(checkCtx, region)
		cancel()
		if err != nil {
			s.l.Debugf("valhalla dependency check failed for region %s: %v", region, err)
			return &healthv1.HealthResponse{Status: healthv1.HealthResponse_DOWN}, nil
		}
	}

	return &healthv1.HealthResponse{Status: healthv1.HealthResponse_UP}, nil
}
