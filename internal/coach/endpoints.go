package coach

import (
	"net/url"
	"regexp"
	"strings"
)

var versionSegmentRe = regexp.MustCompile(`^v\d+([a-zA-Z0-9_\.-]*)$`)

func isVersionSegment(s string) bool {
	return versionSegmentRe.MatchString(s)
}

// NormalizeOpenAIBase normalizes an OpenAI-compatible endpoint URL so it includes
// a version segment (e.g. /v1) and strips any trailing /chat/completions.
func NormalizeOpenAIBase(endpoint string) string {
	trimmed := strings.TrimSpace(endpoint)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return ""
	}

	hasScheme := strings.Contains(trimmed, "://")
	parseURL := trimmed
	if !hasScheme {
		parseURL = "https://" + trimmed
	}

	u, err := url.Parse(parseURL)
	if err != nil {
		return trimmed
	}

	p := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(p, "/chat/completions") {
		p = strings.TrimSuffix(p, "/chat/completions")
		p = strings.TrimRight(p, "/")
	}

	var lastSegment string
	if p != "" {
		parts := strings.Split(p, "/")
		lastSegment = parts[len(parts)-1]
	}

	if !isVersionSegment(lastSegment) {
		p = p + "/v1"
	}
	u.Path = p
	u.RawPath = ""

	result := u.String()
	if !hasScheme {
		result = strings.TrimPrefix(result, "https://")
	}
	return result
}

// OpenAIChatURL returns the full chat completions URL for an OpenAI-compatible provider.
func OpenAIChatURL(endpoint string) string {
	base := NormalizeOpenAIBase(endpoint)
	return appendPathWithQuery(base, "/chat/completions")
}

// OpenAIModelsURL returns the models listing URL for an OpenAI-compatible provider.
func OpenAIModelsURL(endpoint string) string {
	base := NormalizeOpenAIBase(endpoint)
	return appendPathWithQuery(base, "/models")
}

// NormalizeAnthropicBase normalizes an Anthropic endpoint URL so it includes /v1
// and strips any trailing /messages.
func NormalizeAnthropicBase(endpoint string) string {
	trimmed := strings.TrimSpace(endpoint)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "https://api.anthropic.com/v1"
	}

	hasScheme := strings.Contains(trimmed, "://")
	parseURL := trimmed
	if !hasScheme {
		parseURL = "https://" + trimmed
	}

	u, err := url.Parse(parseURL)
	if err != nil {
		return trimmed
	}

	p := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(p, "/messages") {
		p = strings.TrimSuffix(p, "/messages")
		p = strings.TrimRight(p, "/")
	}

	var lastSegment string
	if p != "" {
		parts := strings.Split(p, "/")
		lastSegment = parts[len(parts)-1]
	}

	if lastSegment != "v1" {
		p = p + "/v1"
	}
	u.Path = p
	u.RawPath = ""

	result := u.String()
	if !hasScheme {
		result = strings.TrimPrefix(result, "https://")
	}
	return result
}

// AnthropicMessagesURL returns the Messages endpoint URL for Anthropic.
func AnthropicMessagesURL(endpoint string) string {
	base := NormalizeAnthropicBase(endpoint)
	return appendPathWithQuery(base, "/messages")
}

// AnthropicModelsURL returns the models endpoint URL for Anthropic.
func AnthropicModelsURL(endpoint string) string {
	base := NormalizeAnthropicBase(endpoint)
	return appendPathWithQuery(base, "/models")
}

// NormalizeGeminiBase normalizes a Gemini endpoint URL so it ends in /v1beta or /v1.
func NormalizeGeminiBase(endpoint string) string {
	trimmed := strings.TrimSpace(endpoint)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "https://generativelanguage.googleapis.com/v1beta"
	}

	hasScheme := strings.Contains(trimmed, "://")
	parseURL := trimmed
	if !hasScheme {
		parseURL = "https://" + trimmed
	}

	u, err := url.Parse(parseURL)
	if err != nil {
		return trimmed
	}

	p := strings.TrimRight(u.Path, "/")
	var lastSegment string
	if p != "" {
		parts := strings.Split(p, "/")
		lastSegment = parts[len(parts)-1]
	}

	if lastSegment != "v1beta" && lastSegment != "v1" {
		p = p + "/v1beta"
	}
	u.Path = p
	u.RawPath = ""

	result := u.String()
	if !hasScheme {
		result = strings.TrimPrefix(result, "https://")
	}
	return result
}

// GeminiStreamURL returns the streamGenerateContent URL for Gemini SSE streaming.
func GeminiStreamURL(endpoint, model, apiKey string) string {
	base := NormalizeGeminiBase(endpoint)
	model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
	if model == "" {
		model = "gemini-3.8-flash"
	}

	subpath := "/models/" + url.PathEscape(model) + ":streamGenerateContent"
	return appendGeminiParams(base, subpath, apiKey, true)
}

// GeminiGenerateURL returns the non-streaming generateContent URL used by the
// Settings connectivity probe.
func GeminiGenerateURL(endpoint, model, apiKey string) string {
	base := NormalizeGeminiBase(endpoint)
	model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
	if model == "" {
		model = "gemini-3.8-flash"
	}

	subpath := "/models/" + url.PathEscape(model) + ":generateContent"
	return appendGeminiParams(base, subpath, apiKey, false)
}

// GeminiModelsURL returns the models endpoint URL for Gemini.
func GeminiModelsURL(endpoint, apiKey string) string {
	base := NormalizeGeminiBase(endpoint)
	return appendGeminiParams(base, "/models", apiKey, false)
}

func appendGeminiParams(base, subpath, apiKey string, isStream bool) string {
	base = strings.TrimRight(base, "/")
	hasScheme := strings.Contains(base, "://")
	parseURL := base
	if !hasScheme {
		parseURL = "https://" + base
	}

	u, err := url.Parse(parseURL)
	if err != nil {
		res := base + subpath
		sep := "?"
		if strings.Contains(res, "?") {
			sep = "&"
		}
		if isStream {
			res += sep + "alt=sse"
			sep = "&"
		}
		if apiKey != "" {
			res += sep + "key=" + url.QueryEscape(apiKey)
		}
		return res
	}

	rawQuery := u.RawQuery
	u.RawQuery = ""

	baseNoQuery := strings.TrimRight(u.String(), "/")
	if !hasScheme {
		baseNoQuery = strings.TrimPrefix(baseNoQuery, "https://")
	}
	res := baseNoQuery + subpath

	var queryItems []string
	if rawQuery != "" {
		queryItems = append(queryItems, rawQuery)
	}
	if isStream {
		queryItems = append(queryItems, "alt=sse")
	}
	if apiKey != "" {
		queryItems = append(queryItems, "key="+url.QueryEscape(apiKey))
	}

	if len(queryItems) > 0 {
		res += "?" + strings.Join(queryItems, "&")
	}
	return res
}

// OllamaChatURL returns the chat endpoint URL for Ollama.
func OllamaChatURL(endpoint string) string {
	return appendOllamaPath(endpoint, "/chat")
}

// OllamaTagsURL returns the tags endpoint URL for Ollama.
func OllamaTagsURL(endpoint string) string {
	return appendOllamaPath(endpoint, "/tags")
}

func appendOllamaPath(endpoint, subpath string) string {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		trimmed = "http://localhost:11434"
	}
	trimmed = strings.TrimRight(trimmed, "/")

	hasScheme := strings.Contains(trimmed, "://")
	parseURL := trimmed
	if !hasScheme {
		parseURL = "http://" + trimmed
	}

	u, err := url.Parse(parseURL)
	if err != nil {
		base := strings.TrimSuffix(trimmed, "/api")
		return base + "/api" + subpath
	}

	p := strings.TrimRight(u.Path, "/")
	p = strings.TrimSuffix(p, "/api")
	u.Path = p + "/api" + subpath
	u.RawPath = ""

	res := u.String()
	if !hasScheme {
		res = strings.TrimPrefix(res, "http://")
	}
	return res
}

func appendPathWithQuery(baseURL, subpath string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return ""
	}
	hasScheme := strings.Contains(baseURL, "://")
	parseURL := baseURL
	if !hasScheme {
		parseURL = "https://" + baseURL
	}

	u, err := url.Parse(parseURL)
	if err != nil {
		if strings.Contains(baseURL, "?") {
			parts := strings.SplitN(baseURL, "?", 2)
			return strings.TrimRight(parts[0], "/") + subpath + "?" + parts[1]
		}
		return baseURL + subpath
	}

	u.Path = strings.TrimRight(u.Path, "/") + subpath
	u.RawPath = ""

	res := u.String()
	if !hasScheme {
		res = strings.TrimPrefix(res, "https://")
	}
	return res
}
