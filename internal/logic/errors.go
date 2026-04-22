package logic

import "errors"

var (
	ErrLocationOutsideOfKnownRegions = errors.New("Location outside of known regions")
	ErrNoRouteFound                  = errors.New("No route found")
	ErrValhallaUnavailable           = errors.New("Valhalla backend unavailable")
)
