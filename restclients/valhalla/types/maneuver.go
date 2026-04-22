package types

type ManeuverKind int

const (
	NoneManeuver ManeuverKind = iota
	StartManeuver
	StartRightManeuver
	StartLeftManeuver
	DestinationManeuver
	DestinationRightManeuver
	DistinationLeftManeuver
	BecomesManeuver
	ContinueManeuver
	SlightRightManeuver
	RightManeuver						// 10
	SharpRightManeuver
	UturnRightManeuver
	UturnLeftManeuver
	SharpLeftManeuver
	LeftManeuver
	SlightLeftManeuver
	RampStraightManeuver
	RampRightManeuver
	RampLeftManeuver
	ExitRightManeuver					// 20
	ExitLeftManeuver
	StayStraightManeuver
	StayRightManeuver
	StayLeftManeuver
	MergeManeuver
	RoundaboutEnterManeuver
	RoundaboutExitManeuver
	FerryEnterManeuver
	FerryExitManeuver
	TransitManeuver						// 30
	TransitTransferManeuver
	TransitRemainOnManeuver
	TransitConnectionStartManeuver
	TransitConnectionTransferManeuver
	TransitConnectionDestinationManeuver
	PostTransitConnectionDestinationManeuver
	MergeRightManeuver
	MergeLeftManeuver
	ElevatorEnterManeuver
	StepsEnterManeuver					// 40
	EscalatorEnterManeuver
	BuildingEnterManeuver
	BuildingExitManeuver
)

type Maneuver struct {
	Type								ManeuverKind	`json:"type"`

	// Written maneuver instruction. Describes the maneuver, such as "Turn right
	// onto Main Street".
	Instruction							string			`json:"instruction"`

	// Text suitable for use as a verbal alert in a navigation application.
	// The transition alert instruction will prepare the user for the forthcoming
	// transition.
	// For example: "Turn right onto North Prince Street".
	VerbalTransitionAlertInstruction	string			`json:"verbal_transition_alert_instruction"`

	// Text suitable for use as a verbal message immediately prior to the
	// maneuver transition.
	// For example "Turn right onto North Prince Street, U.S. 2 22".
	VerbalPreTransitionInstruction		string			`json:"verbal_pre_transition_instruction"`

	// Text suitable for use as a verbal message immediately after the maneuver
	// transition.
	// For example "Continue on U.S. 2 22 for 3.9 miles".
	VerbalPostTransitionInstruction		string			`json:"verbal_post_transition_instruction"`

	// List of street names that are consistent along the entire nonobvious
	// maneuver.
	StreetNames							[]string		`json:"street_names,omitempty"`

	// When present, these are the street names at the beginning (transition
	// point) of the nonobvious maneuver (if they are different than the names
	// that are consistent along the entire nonobvious maneuver).
	BeginStreetNames					[]string		`json:"begin_street_names,omitempty"`

	// Estimated time along the maneuver in seconds.
	Time								float64			`json:"time"`

	// Maneuver length in the units specified.
	Length								float64			`json:"length"`

	// Index into the list of shape points for the start of the maneuver.
	BeginShapeIndex						int				`json:"begin_shape_index"`

	// Index into the list of shape points for the end of the maneuver.
	EndShapeIndex						int				`json:"end_shape_index"`

	// True if the maneuver has any toll, or portions of the maneuver are subject
	// to a toll.
	Toll								*bool			`json:"toll,omitempty"`

	// True if a highway is encountered on this maneuver.
	Highway								*bool			`json:"highway,omitempty"`

	// True if the maneuver is unpaved or rough pavement, or has any portions
	// that have rough pavement.
	Rough								*bool			`json:"rough,omitempty"`

	// True if a gate is encountered on this maneuver.
	Gate								*bool			`json:"gate,omitempty"`

	// True if a ferry is encountered on this maneuver.
	Ferry								*bool			`json:"ferry,omitempty"`

	Sign								*Sign			`json:"sign,omitempty"`

	// The spoke to exit roundabout after entering.
	RoundaboutExitCount					*int			`json:"roundabout_exit_count,omitempty"`

	// Written depart time instruction.
	// Typically used with a transit maneuver,
	// such as "Depart: 8:04 AM from 8 St - NYU".
	DepartInstruction					*string			`json:"depart_instruction,omitempty"`

	// Text suitable for use as a verbal depart time instruction.
	// Typically used with a transit maneuver,
	// such as "Depart at 8:04 AM from 8 St - NYU".
	VerbalDepartInstruction				*string			`json:"verbal_depart_instruction,omitempty"`

	// Written arrive time instruction.
	// Typically used with a transit maneuver,
	// such as "Arrive: 8:10 AM at 34 St - Herald Sq".
	ArriveInstruction					*string			`json:"arrive_instruction,omitempty"`

	// Text suitable for use as a verbal arrive time instruction.
	// Typically used with a transit maneuver,
	// such as "Arrive at 8:10 AM at 34 St - Herald Sq".
	VerbalArriveInstruction				*string			`json:"verbal_arrive_instruction,omitempty"`

	// Contains the attributes that describe a specific transit route.
	TransitInfo							*TransitInfo	`json:"transit_info,omitempty"`

	// True if the verbal_pre_transition_instruction has been appended with the
	// verbal instruction of the next maneuver.
	VerbalMultiCue						*bool			`json:"verbal_multi_cue,omitempty"`

	TravelMode							TravelMode		`json:"travel_mode"`
	TravelType							TravelType		`json:"travel_type"`

	// Used when travel_mode is bikeshare.
	// Describes bike share maneuver. The default value is "NoneAction 
	BssManeuverType						*BikeShareManeuver	`json:"bss_maneuver_type,omitempty"`

	// The clockwise angle from true north to the direction of travel immediately
	// before the maneuver.
	BearingBefore						int				`json:"bearing_before"`

	//  	The clockwise angle from true north to the direction of travel
	// immediately after the maneuver.
	BearingAfter						int				`json:"bearing_after"`

	// An array describing lane-level guidance.
	// Used when turn_lanes is enabled. See below for details.
	Lanes								[]TurnLane		`json:"lanes,omitempty"`
}

