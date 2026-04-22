package types

type TurnDirectionMask = uint16

const (
	TDEmpty			TurnDirectionMask = 0x0000
	TDNone			TurnDirectionMask = 0x0001
	TDThrough		TurnDirectionMask = 0x0002
	TDSharpLeft		TurnDirectionMask = 0x0004
	TDLeft			TurnDirectionMask = 0x0008
	TDSlightLeft	TurnDirectionMask = 0x0010
	TDSlightRight	TurnDirectionMask = 0x0020
	TDRight			TurnDirectionMask = 0x0040
	TDSharpRight	TurnDirectionMask = 0x0080
	TDReverse		TurnDirectionMask = 0x0100
	TDMergeToLeft	TurnDirectionMask = 0x0200
	TDMergeToRight	TurnDirectionMask = 0x0400
)

type TurnLane struct {
	// A bitmask indicating all possible turn directions for that lane.
	Directions 		TurnDirectionMask			`json:"directions"`

	// A bitmask indicating valid turn directions for following the route
	// initially.
	// A lane is marked valid if it can be used at the start of the maneuver but
	// might require further lane changes.
	Valid			*TurnDirectionMask			`json:"valid,omitempty"`

	// A bitmask indicating active turn directions for continuing along the route
	// without needing additional lane changes.
	// A lane is marked active if it is the best lane for following the maneuver
	// as intended.
	Active			*TurnDirectionMask			`json:"active,omitempty"`
}
