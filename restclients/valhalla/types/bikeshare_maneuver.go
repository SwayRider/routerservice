package types

type BikeShareManeuver string

const (
	NoneAction 					BikeShareManeuver = "NoneAction"
	RentBikeAtBikeShare        	BikeShareManeuver = "RentBikeAtBikeShare"
	ReturnBikeAtBikeShare      	BikeShareManeuver = "ReturnBikeAtBikeShare"
)
