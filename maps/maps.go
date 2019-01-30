package maps

import "fmt"

// Provider is a provider for a map.
type Provider int

const (
	// GoogleMaps is the provider using Google Maps
	GoogleMaps Provider = iota
	// OpenStreetMap is the provider using OpenStreetMap.org
	OpenStreetMap
)

// googleMaps returns an URL to the given latitude and longitude on Google Maps.
func googleMaps(latitude, longitude float64) string {
	return fmt.Sprintf(
		"https://maps.google.com/?q=%f,%f",
		latitude,
		longitude,
	)
}

// openStreetMap returns an URL to the given latitude and longitude on
// OpenStreetMap.org.
func openStreetMap(latitude, longitude float64) string {
	return fmt.Sprintf(
		"https://www.openstreetmap.org/#map=19/%f/%f",
		latitude,
		longitude,
	)
}

// ByProvider returns an URL to the given latitude and longitude on the given
// provider.
func ByProvider(provider Provider, latitude, longitude float64) string {
	switch provider {
	case OpenStreetMap:
		return openStreetMap(latitude, longitude)
	}

	return googleMaps(latitude, longitude)
}
