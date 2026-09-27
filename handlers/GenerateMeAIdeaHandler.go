package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// https://www.alexedwards.net/blog/how-to-rate-limit-http-requests
var visitors = make(map[string]*rate.Limiter)
var mu sync.Mutex

func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(time.Hour/10), 5) // 10 requests per hour
		visitors[ip] = limiter
	}

	return limiter
}

func getClientIP(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "CF-Connecting-IP", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}

		if header == "X-Forwarded-For" {
			parts := strings.Split(value, ",")
			if len(parts) > 0 {
				first := strings.TrimSpace(parts[0])
				if first != "" {
					return first
				}
			}
		}

		return value
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	return r.RemoteAddr
}

var allowedStuff = map[int][]map[string]string{
	1: {
		{"name": "Website", "emoji": "🌐"},
		{"name": "TUI", "emoji": "🖥️"},
		{"name": "GUI", "emoji": "🖼️"},
		{"name": "CLI", "emoji": "⌨️"},
		{"name": "API", "emoji": "🔌"},
		{"name": "Bot", "emoji": "🤖"},
	},
	2: {
		{"name": "Games", "emoji": "🎮"},
		{"name": "Utilities", "emoji": "🧰"},
		{"name": "Demos", "emoji": "🧪"},
		{"name": "Simulations", "emoji": "🧬"},
		{"name": "Scrapers", "emoji": "🕸️"},
		{"name": "Hardware", "emoji": "⚙️"},
	},
	3: {
		{"name": "Learning", "emoji": "📚"},
		{"name": "Fun", "emoji": "🎉"},
		{"name": "Productivity", "emoji": "📈"},
		{"name": "Chaos", "emoji": "🌪️"},
		{"name": "Art", "emoji": "🎨"},
		{"name": "Community", "emoji": "🫂"},
	},
}

// shoutout ai.hackclub.com
func GenerateMeAIdeaHandler(w http.ResponseWriter, r *http.Request) {
	// if GET, return the slot options, else if POST, generate an idea
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(allowedStuff)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	AI_BASE_URL := "https://ai.hackclub.com/proxy/v1/chat/completions"
	AI_API_TOKEN := os.Getenv("AI_API_TOKEN")
	AI_MODEL := "z-ai/glm-5.3-flashx"

	if AI_BASE_URL == "" || AI_API_TOKEN == "" {
		http.Error(w, "AI API not configured", 500)
		return
	}

	// make sure the request follows the format of {slot1: string, slot2: string, slot3: string}, and those contain allowed options
	var reqData struct {
		Slot1 string `json:"type"`
		Slot2 string `json:"category"`
		Slot3 string `json:"theme"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		http.Error(w, "Invalid request body", 400)
		return
	}

	// validate
	validateSlot := func(slot string, options []map[string]string) bool {
		for _, option := range options {
			if slot == option["name"] {
				return true
			}
		}
		return false
	}

	if !validateSlot(reqData.Slot1, allowedStuff[1]) || !validateSlot(reqData.Slot2, allowedStuff[2]) || !validateSlot(reqData.Slot3, allowedStuff[3]) {
		http.Error(w, "Invalid slot values", 400)
		return
	}

	prompt := `Generate a interesting and unique 100% Golang project idea for beginner programmers with this theme (randomly selected):\n\n
              Type: ` + reqData.Slot1 +
		`\nCategory: ` + reqData.Slot2 +
		`\nTheme: ` + reqData.Slot3 +
		`\n\nMake sure the idea is not too common, and is something that can be built in a few days to a week. Also make sure to include some fun details to make the idea more interesting.
			  Keep it under 500 characters, make sure to include which packages should be used, don't use any formatting.`

	// first off, ratelimit of whatever reqs per hour per ip
	ip := getClientIP(r)
	limiter := getVisitor(ip)
	if !limiter.Allow() {
		http.Error(w, "429 Too many requests", http.StatusTooManyRequests)
		return
	}

	// cook the request to ai.hackclub.com
	req, err := http.NewRequest("POST", AI_BASE_URL, nil)
	if err != nil {
		http.Error(w, "Failed to create AI request", 500)
		return
	}
	req.Header.Set("Authorization", "Bearer "+AI_API_TOKEN)
	req.Header.Set("Content-Type", "application/json")
	req.Body =
		func() io.ReadCloser {
			b, _ := json.Marshal(map[string]any{
				"model": AI_MODEL,
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
			})
			return io.NopCloser(bytes.NewReader(b))
		}()

	// send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to send AI request", 500)
		return
	}
	defer resp.Body.Close()

	// read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read AI response", 500)
		return
	}

	// get the answer from the response
	var aiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &aiResp); err != nil {
		http.Error(w, "Failed to parse AI response", 500)
		return
	}

	if len(aiResp.Choices) == 0 || aiResp.Choices[0].Message.Content == "" {
		http.Error(w, "AI response missing content", 500)
		return
	}

	// everything passed, send response
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(aiResp.Choices[0].Message.Content))
}
