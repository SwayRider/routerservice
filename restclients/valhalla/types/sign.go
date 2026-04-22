package types

type SignElement struct {
	// Interchange sign text.
    //	- exit number example: 91B.
    //	-exit branch example: I 95 North.
    //	- exit toward example: New York.
    //	- exit name example: Gettysburg Pike.
	Text 				string		`json:"text"`

	// The frequency of this sign element within a set a consecutive signs.
	// This item is optional.
	ConsecutiveCount 	*int		`json:"consecutive_count,omitempty"`
}

type Sign struct {
	// list of exit number elements.
	// If an exit number element exists, it is typically just one value.
	ExitNumberElements []SignElement	`json:"exit_number_elements,omitempty"`

	// list of exit branch elements.
	// The exit branch element text is the subsequent road name or route number
	// after the sign.
	ExitBranchElements []SignElement	`json:"exit_branch_elements,omitempty"`

	// list of exit toward elements.
	// The exit toward element text is the location where the road ahead goes -
	// the location is typically a control city, but may also be a future road
	// name or route number.
	ExitTowardElements []SignElement	`json:"exit_toward_elements,omitempty"`

	// list of exit name elements.
	// The exit name element is the interchange identifier - typically not used
	// in the US.
	ExitNameElements []SignElement	`json:"exit_name_elements,omitempty"`
}
