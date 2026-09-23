package watch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WeatherClient fetches weather forecast without requiring API keys using Open-Meteo.
type WeatherClient struct {
	HTTPClient *http.Client
}

// NewWeatherClient creates a weather client.
func NewWeatherClient() *WeatherClient {
	return &WeatherClient{
		HTTPClient: &http.Client{Timeout: 8 * time.Second},
	}
}

type geocodingResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Country   string  `json:"country"`
	} `json:"results"`
}

type openMeteoResponse struct {
	Daily struct {
		WeatherCode               []int       `json:"weather_code"`
		Temperature2mMax          []float64   `json:"temperature_2m_max"`
		Temperature2mMin          []float64   `json:"temperature_2m_min"`
		PrecipitationProbability []int       `json:"precipitation_probability_max"`
	} `json:"daily"`
}

// GetDailyForecast returns a formatted one-line forecast for the given city (e.g. "Roma").
func (w *WeatherClient) GetDailyForecast(city string) (string, error) {
	city = strings.TrimSpace(city)
	if city == "" {
		city = "Roma"
	}

	// 1. Resolve coordinates via open geocoding API
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=it&format=json", url.QueryEscape(city))
	resp, err := w.HTTPClient.Get(geoURL)
	if err != nil {
		return "", fmt.Errorf("geocoding non raggiungibile: %w", err)
	}
	defer resp.Body.Close()

	var geo geocodingResponse
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil || len(geo.Results) == 0 {
		return "", fmt.Errorf("città non trovata per meteo: %s", city)
	}

	matched := geo.Results[0]

	// 2. Fetch daily forecast
	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max&timezone=auto&forecast_days=1",
		matched.Latitude,
		matched.Longitude,
	)

	fResp, err := w.HTTPClient.Get(forecastURL)
	if err != nil {
		return "", fmt.Errorf("previsioni meteo non raggiungibili: %w", err)
	}
	defer fResp.Body.Close()

	var om openMeteoResponse
	if err := json.NewDecoder(fResp.Body).Decode(&om); err != nil {
		return "", fmt.Errorf("errore parsing previsioni: %w", err)
	}

	if len(om.Daily.WeatherCode) == 0 || len(om.Daily.Temperature2mMax) == 0 || len(om.Daily.Temperature2mMin) == 0 {
		return "", fmt.Errorf("dati meteo incompleti")
	}

	code := om.Daily.WeatherCode[0]
	maxT := om.Daily.Temperature2mMax[0]
	minT := om.Daily.Temperature2mMin[0]

	rainProb := 0
	if len(om.Daily.PrecipitationProbability) > 0 {
		rainProb = om.Daily.PrecipitationProbability[0]
	}

	desc, emoji := interpretWMOWeatherCode(code)

	return fmt.Sprintf("%s <b>%s</b>: %s, Min: <b>%.0f°C</b> / Max: <b>%.0f°C</b> (Prob. pioggia: %d%%)",
		emoji,
		matched.Name,
		desc,
		minT,
		maxT,
		rainProb,
	), nil
}

// interpretWMOWeatherCode maps standard WMO weather codes to Italian descriptions and emojis.
func interpretWMOWeatherCode(code int) (string, string) {
	switch code {
	case 0:
		return "Sereno", "☀️"
	case 1:
		return "Prevalentemente sereno", "🌤️"
	case 2:
		return "Parzialmente nuvoloso", "⛅"
	case 3:
		return "Coperto", "☁️"
	case 45, 48:
		return "Nebbia", "🌫️"
	case 51, 53, 55:
		return "Pioviggine leggera", "🌦️"
	case 61, 63:
		return "Pioggia moderata", "🌧️"
	case 65:
		return "Pioggia intensa", "🌧️"
	case 71, 73, 75:
		return "Neve", "🌨️"
	case 80, 81, 82:
		return "Rovesci di pioggia", "🌧️"
	case 95, 96, 99:
		return "Temporale", "⛈️"
	default:
		return "Variabile", "🌤️"
	}
}
