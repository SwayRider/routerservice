package types

type Administrative struct {
	TimeZonePosix 				string `json:"time_zone_posix"`
	StandardTimeZone 			string `json:"standard_time_zone"`
	ISO_3166_1 					string `json:"iso_3166-1"`
	DaylightSavingsTimeZoneName	string `json:"daylight_savings_time_zone_name"`
	Country 					string `json:"country"`
	ISO_3166_2 					string `json:"iso_3166-2"`
	State 						string `json:"state"`
}
