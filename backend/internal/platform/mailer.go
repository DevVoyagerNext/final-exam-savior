package platform

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"final-exam-savior/backend/internal/config"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}
type SMTPMailer struct {
	cfg config.SMTPConfig
}

func NewSMTPMailer(cfg config.SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}
func (m *SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	if m.cfg.Host == "" || m.cfg.Username == "" || m.cfg.Password == "" || m.cfg.From == "" {
		return fmt.Errorf("smtp config is incomplete")
	}
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	msg := strings.Builder{}
	msg.WriteString(fmt.Sprintf("From: %s\r\n", m.cfg.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(body)
	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(msg.String()))
}
