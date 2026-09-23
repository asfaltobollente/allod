package watch

import (
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
