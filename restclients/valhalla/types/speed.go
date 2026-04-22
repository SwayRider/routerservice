package types

type Speed string

const (
	Freeflow    Speed = "freeflow"
	Constrained Speed = "constrained"
	Predicted   Speed = "predicted"
	Current     Speed = "current"
)
