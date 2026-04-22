package types

type RoadClass string

const (
	Motorway		RoadClass = "motorway"
	Trunk			RoadClass = "trunk"
	Primary			RoadClass = "primary"
	Secondary		RoadClass = "secondary"
	Tertiary		RoadClass = "tertiary"
	Unclassified	RoadClass = "unclassified"
	Residential		RoadClass = "residential"
	Service			RoadClass = "service"
	Track			RoadClass = "track"
)
