package server

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/swayrider/routerservice/internal/logic"
)

func grpcStatus(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, logic.ErrValhallaUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, logic.ErrLocationOutsideOfKnownRegions),
		errors.Is(err, logic.ErrNoRouteFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
