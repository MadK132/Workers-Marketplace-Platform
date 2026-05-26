package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Sender struct {
	apiKey string
	from   string
	apiURL string
	client *http.Client
}

func NewSender(apiKey, from string) *Sender {
	apiURL := strings.TrimSpace(os.Getenv("RESEND_API_URL"))
	if apiURL == "" {
		apiURL = "https://api.resend.com/emails"
	}
	if strings.TrimSpace(from) == "" {
		from = "Workers Marketplace <onboarding@resend.dev>"
	}
	return &Sender{
		apiKey: strings.TrimSpace(apiKey),
		from:   strings.TrimSpace(from),
		apiURL: apiURL,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Sender) SendVerificationEmail(to, token string) error {
	link := buildAppURL("/auth/verify?token=" + token)
	return s.send(to, "Verify your email", `
		<h2>Email Verification</h2>
		<p>Click the link below to verify your account:</p>
		<a href="`+html.EscapeString(link)+`">Verify Email</a>
	`)
}

func (s *Sender) SendResetEmail(to, token string) error {
	link := buildAppURL("/auth/reset?token=" + token)
	return s.send(to, "Reset password", `
		<h2>Password Reset</h2>
		<p>Click the link below to reset your password:</p>
		<a href="`+html.EscapeString(link)+`">Reset Password</a>
	`)
}

func (s *Sender) send(to, subject, htmlBody string) error {
	if s == nil {
		return fmt.Errorf("email sender is not configured")
	}
	if s.apiKey == "" {
		return fmt.Errorf("RESEND_API_KEY is required")
	}
	if s.from == "" {
		return fmt.Errorf("RESEND_FROM is required")
	}

	payload := map[string]any{
		"from":    s.from,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("resend email failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func buildAppURL(path string) string {
	baseURL := os.Getenv("APP_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5173"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}
