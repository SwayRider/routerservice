package types

type TravelMode string

const (
	DriveTravelMode			TravelMode = "drive"
	PedestrianTravelMode	TravelMode = "pedestrian"
	BicycleTravelMode		TravelMode = "bike"
	TransitTravelMode		TravelMode = "transit"
)

type TravelType string

const (
	// Travel types for drive
	CarTravelType				TravelType = "car"
	MotorScooterTravelType		TravelType = "motor_scooter"
	MotorcycleTravelType		TravelType = "motorcycle"
	TruckTravelType				TravelType = "truck"
	BusTravelType				TravelType = "bus"

	// Travel types for pedestrian
	FootTravelType				TravelType = "foot"
	WheelchairTravelType		TravelType = "wheelchair"

	// Travel types for bicycl
	RoadTravelType				TravelType = "road"
	HybridTravelType			TravelType = "hybrid"
	CrossTravelType				TravelType = "cross"
	MountainTravelType			TravelType = "mountain"

	// Travel types for transit
	TramTravelType				TravelType = "tram"
	MetroTravelType				TravelType = "metro"
	RailTravelType				TravelType = "rail"
	//BusTravelType				TravelType = "bus"			// Also valid for transit
	FerryTravelType				TravelType = "ferry"
	CableCarTravelType			TravelType = "cable_car"
	GondolaTravelType			TravelType = "gondola"
	FunicularTravelType			TravelType = "funicular"
)
