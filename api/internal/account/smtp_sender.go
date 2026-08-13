package account

import (
	"context"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
)

type SMTPSettings struct {
	Address  string
	From     string
	Username string
	Password string
}

type SMTPSender struct {
	settings SMTPSettings
	from     *mail.Address
	host     string
}

func NewSMTPSender(settings SMTPSettings) (*SMTPSender, error) {
	host, _, err := net.SplitHostPort(settings.Address)
	if err != nil {
		return nil, fmt.Errorf("SMTP_ADDR must include a host and port: %w", err)
	}
	from, err := mail.ParseAddress(settings.From)
	if err != nil {
		return nil, fmt.Errorf("SMTP_FROM is not an email address: %w", err)
	}
	return &SMTPSender{settings: settings, from: from, host: host}, nil
}

func (s *SMTPSender) SendVerification(_ context.Context, address, link string) error {
	var auth smtp.Auth
	if s.settings.Username != "" {
		auth = smtp.PlainAuth("", s.settings.Username, s.settings.Password, s.host)
	}
	recipient := (&mail.Address{Address: address}).String()
	message := "From: " + s.from.String() + "\r\n" +
		"To: " + recipient + "\r\n" +
		"Subject: Verify your LumiHub email\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		"Verify your LumiHub email address by opening this link:\r\n\r\n" + link + "\r\n"

	if err := smtp.SendMail(
		s.settings.Address,
		auth,
		s.from.Address,
		[]string{address},
		[]byte(message),
	); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}
	return nil
}
