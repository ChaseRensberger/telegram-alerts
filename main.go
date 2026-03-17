package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

type SendRequest struct {
	Message string `json:"message"`
}

func main() {
	godotenv.Load(".env.local")
	botToken := mustEnv("TELEGRAM_BOT_TOKEN")
	chatID := mustEnv("TELEGRAM_CHAT_ID")
	apiKey := mustEnv("API_KEY")

	port := os.Getenv("PORT")
	if port == "" {
		port = "1323"
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.With(authMiddleware(apiKey)).Post("/send", sendHandler(botToken, chatID))

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

func authMiddleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == header || token != apiKey {
				http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func sendHandler(botToken, chatID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, `{"error":"message is required"}`, http.StatusBadRequest)
			return
		}

		if err := sendTelegramMessage(botToken, chatID, req.Message); err != nil {
			log.Printf("telegram error: %v", err)
			http.Error(w, `{"error":"failed to send telegram message"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
	}
}

func sendTelegramMessage(botToken, chatID, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload, err := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    message,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("post to telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		return fmt.Errorf("telegram returned %d: %v", resp.StatusCode, body)
	}

	return nil
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return val
}
