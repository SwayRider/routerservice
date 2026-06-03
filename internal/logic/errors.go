package logic

import "errors"

var (
	ErrLocationOutsideOfKnownRegions = errors.New("location outside of known regions")
	ErrNoRouteFound                  = errors.New("no route found")
	ErrValhallaUnavailable           = errors.New("valhalla backend unavailable")
)
