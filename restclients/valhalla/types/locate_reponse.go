package types

type LocateResponse struct {
	InputLon  	float64 	`json:"input_lon"`
	InputLat  	float64 	`json:"input_lat"`
	Nodes 		[]Node 		`json:"nodes"`
	Edges 		[]Edge 		`json:"edges"`
	Warnings 	[]string	`json:"warnings"`
}
