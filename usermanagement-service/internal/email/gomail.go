package email

import (
	"os"
	"strings"

	"gopkg.in/gomail.v2"
)

type Sender struct {
	host string
	port int
	user string
	pass string
}

func NewSender(host string, port int, user, pass string) *Sender {
	return &Sender{host, port, user, pass}
}

func (s *Sender) SendVerificationEmail(to, token string) error {
	m := gomail.NewMessage()

	m.SetHeader("From", s.user)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Verify your email")

	link := buildAppURL("/auth/verify?token=" + token)

	m.SetBody("text/html", `
		<h2>Email Verification</h2>
		<p>Click the link below to verify your account:</p>
		<a href="`+link+`">Verify Email</a>
	`)

	d := gomail.NewDialer(s.host, s.port, s.user, s.pass)

	return d.DialAndSend(m)
}
func (s *Sender) SendResetEmail(to, token string) error {
	m := gomail.NewMessage()

	m.SetHeader("From", s.user)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Reset password")

	link := buildAppURL("/auth/reset?token=" + token)

	m.SetBody("text/html", `
		<h2>Password Reset</h2>
		<p>Click the link below to reset your password:</p>
		<a href="`+link+`">Reset Password</a>
	`)

	d := gomail.NewDialer(s.host, s.port, s.user, s.pass)

	return d.DialAndSend(m)
}

func buildAppURL(path string) string {
	baseURL := os.Getenv("APP_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}
