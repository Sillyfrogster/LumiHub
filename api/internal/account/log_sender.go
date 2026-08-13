package account

import (
	"context"
	"log"
)

type LogVerificationSender struct {
	logger *log.Logger
}

func NewLogVerificationSender(logger *log.Logger) *LogVerificationSender {
	return &LogVerificationSender{logger: logger}
}

func (s *LogVerificationSender) SendVerification(_ context.Context, _ string, link string) error {
	s.logger.Printf("email verification link: %s", link)
	return nil
}
