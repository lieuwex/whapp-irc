package maps

import "fmt"

type Provider int

const (
	GoogleMaps Provider = iota
	OpenStreetMap
)

// googleMaps returns a URL to the given latitude and longitude on Google Maps.
func googleMaps(latitude, longitude float64) string {
	return fmt.Sprintf(
		"https://maps.google.com/?q=%f,%f",
		latitude,
		longitude,
	)
}

// openStreetMap returns a URL to the given latitude and longitude on
// OpenStreetMap.org.
func openStreetMap(latitude, longitude float64) string {
	return fmt.Sprintf(
		"https://www.openstreetmap.org/#map=19/%f/%f",
		latitude,
		longitude,
	)
}

func ByProvider(provider Provider, latitude, longitude float64) string {
	switch provider {
	case OpenStreetMap:
		return openStreetMap(latitude, longitude)
	}

	return googleMaps(latitude, longitude)
}
