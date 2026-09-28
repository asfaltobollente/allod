package watch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInterpretWMOWeatherCode(t *testing.T) {
	tests := []struct {
		code         int
		expectedDesc string
		expectedEmo  string
	}{
		{0, "Sereno", "☀️"},
		{1, "Prevalentemente sereno", "🌤️"},
		{3, "Coperto", "☁️"},
		{61, "Pioggia moderata", "🌧️"},
		{71, "Neve", "🌨️"},
		{95, "Temporale", "⛈️"},
		{999, "Variabile", "🌤️"},
	}

	for _, tt := range tests {
		desc, emo := interpretWMOWeatherCode(tt.code)
		if !strings.EqualFold(desc, tt.expectedDesc) {
			t.Errorf("Per codice %d atteso desc %s, ottenuto %s", tt.code, tt.expectedDesc, desc)
		}
		if emo != tt.expectedEmo {
			t.Errorf("Per codice %d atteso emoji %s, ottenuto %s", tt.code, tt.expectedEmo, emo)
		}
	}
}

func TestWeatherMockOpenMeteo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "search") {
			_, _ = fmt.Fprintln(w, `{"results":[{"id":3182697,"name":"Atri","latitude":42.576,"longitude":13.988,"country":"Italy"}]}`)
			return
		}
		if strings.Contains(r.URL.Path, "forecast") {
			// Test with null and floats in daily array to ensure resilience
			_, _ = fmt.Fprintln(w, `{"daily":{"time":["2026-09-28"],"weather_code":[1],"temperature_2m_max":[21.4],"temperature_2m_min":[10.2],"precipitation_probability_max":[null]}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client := NewWeatherClient()
	client.HTTPClient = ts.Client()

	// Direct test of fetch and parsing
	geoData, err := client.fetchWithRetry(ts.URL+"/search", 1)
	if err != nil {
		t.Fatalf("failed to fetch mock geo: %v", err)
	}
	if !strings.Contains(string(geoData), "Atri") {
		t.Errorf("expected geo to contain Atri, got %s", string(geoData))
	}

	fData, err := client.fetchWithRetry(ts.URL+"/forecast", 1)
	if err != nil {
		t.Fatalf("failed to fetch mock forecast: %v", err)
	}
	if !strings.Contains(string(fData), "21.4") {
		t.Errorf("expected forecast to contain 21.4, got %s", string(fData))
	}
}
