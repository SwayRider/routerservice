package types

type ShapeFormat string

const (
	PolyLine6	ShapeFormat = "polyline6"
	PolyLine5	ShapeFormat = "polyline5"
	GeoJson		ShapeFormat = "geojson"
	NoShape		ShapeFormat = "no_shape"
)
