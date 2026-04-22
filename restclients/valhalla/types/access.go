package types

type Access struct {
	Wheelchair bool `json:"wheelchair"`
	Taxi       bool `json:"taxi"`
	HOV        bool `json:"HOV"`
	Truck      bool `json:"truck"`
	Emergency  bool `json:"emergency"`
	Pedestrian bool `json:"pedestrian"`
	Car        bool `json:"car"`
	Bus        bool `json:"bus"`
	Bicycle    bool `json:"bicycle"`
}
