package types

type NodeId struct {
	Id 		int `json:"id"`
	Value 	int `json:"value"`
	TileId	int `json:"tile_id"`
	Level 	int `json:"level"`
}

type Node struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`

	TrafficSignal 		*bool 			`json:"traffic_signal,omitempty"`
	Type 				*string 		`json:"type,omitempty"`
	NodeId 				*NodeId 		`json:"node_id,omitempty"`
	Access 				*Access 		`json:"access,omitempty"`
	EdgeCount 			*int 			`json:"edge_count,omitempty"`
	Administrative 		*Administrative	`json:"administrative,omitempty"`
	IntersectionType	*string 		`json:"intersection_type,omitempty"`
	Density 			*int 			`json:"density,omitempty"`
	LocalEdgeCount 		*int 			`json:"local_edge_count,omitempty"`
	ModeChange 			*bool 			`json:"mode_change,omitempty"`
}
