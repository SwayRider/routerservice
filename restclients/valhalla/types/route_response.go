package types

type RouteResponse struct {
	Id					*string					`json:"id,omitempty"`
	Trip 				Trip					`json:"trip"`
}

