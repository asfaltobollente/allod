package watch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WeatherClient fetches weather forecast without requiring API keys using Open-Meteo with wttr.in fallback.
type WeatherClient struct {
	HTTPClient *http.Client
}

// NewWeatherClient creates a resilient weather client with a 15-second timeout.
func NewWeatherClient() *WeatherClient {
	return &WeatherClient{
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
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
		WeatherCode               []any `json:"weather_code"`
		Temperature2mMax          []any `json:"temperature_2m_max"`
		Temperature2mMin          []any `json:"temperature_2m_min"`
		PrecipitationProbability []any `json:"precipitation_probability_max"`
	} `json:"daily"`
}

func parseNumber(val any, defaultVal float64) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	}
	return defaultVal
}

func (w *WeatherClient) fetchWithUserAgent(reqURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Allod-Sentinel/1.0 (+https://github.com/asfaltobollente/allod)")
	req.Header.Set("Accept", "application/json")

	resp, err := w.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return io.ReadAll(resp.Body)
}

func (w *WeatherClient) fetchWithRetry(reqURL string, maxRetries int) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		data, err := w.fetchWithUserAgent(reqURL)
		if err == nil && len(data) > 0 {
			return data, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (w *WeatherClient) getOpenMeteoForecast(city string) (string, error) {
	// 1. Resolve coordinates via open geocoding API
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=it&format=json", url.QueryEscape(city))
	geoData, err := w.fetchWithRetry(geoURL, 3)
	if err != nil {
		return "", fmt.Errorf("geocoding non raggiungibile: %w", err)
	}

	var geo geocodingResponse
	if err := json.Unmarshal(geoData, &geo); err != nil || len(geo.Results) == 0 {
		return "", fmt.Errorf("città non trovata per meteo: %s", city)
	}

	matched := geo.Results[0]

	// 2. Fetch daily forecast
	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max&timezone=auto&forecast_days=1",
		matched.Latitude,
		matched.Longitude,
	)

	fData, err := w.fetchWithRetry(forecastURL, 3)
	if err != nil {
		return "", fmt.Errorf("previsioni meteo non raggiungibili: %w", err)
	}

	var om openMeteoResponse
	if err := json.Unmarshal(fData, &om); err != nil {
		return "", fmt.Errorf("errore parsing previsioni: %w", err)
	}

	if len(om.Daily.WeatherCode) == 0 || len(om.Daily.Temperature2mMax) == 0 || len(om.Daily.Temperature2mMin) == 0 {
		return "", fmt.Errorf("dati meteo incompleti")
	}

	code := int(parseNumber(om.Daily.WeatherCode[0], 0))
	maxT := parseNumber(om.Daily.Temperature2mMax[0], 20)
	minT := parseNumber(om.Daily.Temperature2mMin[0], 10)

	rainProb := 0
	if len(om.Daily.PrecipitationProbability) > 0 && om.Daily.PrecipitationProbability[0] != nil {
		rainProb = int(parseNumber(om.Daily.PrecipitationProbability[0], 0))
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

func (w *WeatherClient) getWttrInForecast(city string) (string, error) {
	reqURL := fmt.Sprintf("https://wttr.in/%s?format=j1", url.PathEscape(city))
	body, err := w.fetchWithRetry(reqURL, 2)
	if err != nil {
		// Try format 3 plain text fallback
		reqURLPlain := fmt.Sprintf("https://wttr.in/%s?format=%%C+%%t+(min+%%f,+max+%%M)&m", url.PathEscape(city))
		if data, pErr := w.fetchWithUserAgent(reqURLPlain); pErr == nil && len(data) > 0 {
			clean := strings.TrimSpace(string(data))
			if clean != "" && !strings.Contains(clean, "Unknown location") {
				return fmt.Sprintf("🌤️ <b>%s</b>: %s", city, clean), nil
			}
		}
		return "", err
	}

	var wttr struct {
		CurrentCondition []struct {
			TempC       string `json:"temp_C"`
			WeatherDesc []struct {
				Value string `json:"value"`
			} `json:"weatherDesc"`
		} `json:"current_condition"`
		Weather []struct {
			MaxtempC string `json:"maxtempC"`
			MintempC string `json:"mintempC"`
			Hourly   []struct {
				Chanceofrain string `json:"chanceofrain"`
			} `json:"hourly"`
		} `json:"weather"`
	}

	if err := json.Unmarshal(body, &wttr); err != nil {
		return "", err
	}

	if len(wttr.Weather) == 0 {
		return "", fmt.Errorf("nessuna previsione wttr.in trovata")
	}

	desc := "Sereno"
	if len(wttr.CurrentCondition) > 0 && len(wttr.CurrentCondition[0].WeatherDesc) > 0 {
		desc = wttr.CurrentCondition[0].WeatherDesc[0].Value
	}
	minT := wttr.Weather[0].MintempC
	maxT := wttr.Weather[0].MaxtempC
	rainProb := "0"
	if len(wttr.Weather[0].Hourly) > 0 && wttr.Weather[0].Hourly[0].Chanceofrain != "" {
		rainProb = wttr.Weather[0].Hourly[0].Chanceofrain
	}

	return fmt.Sprintf("🌤️ <b>%s</b>: %s, Min: <b>%s°C</b> / Max: <b>%s°C</b> (Prob. pioggia: %s%%)",
		city, desc, minT, maxT, rainProb), nil
}

// GetDailyForecast returns a formatted one-line forecast for the given city (e.g. "Atri", "Roma").
func (w *WeatherClient) GetDailyForecast(city string) (string, error) {
	city = strings.TrimSpace(city)
	if city == "" {
		city = "Roma"
	}

	forecast, err := w.getOpenMeteoForecast(city)
	if err == nil && forecast != "" {
		return forecast, nil
	}

	// Fallback to wttr.in if Open-Meteo had transient network or parsing issues
	if wttrForecast, wErr := w.getWttrInForecast(city); wErr == nil && wttrForecast != "" {
		return wttrForecast, nil
	}

	return "", fmt.Errorf("servizi meteo non disponibili: %v", err)
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
