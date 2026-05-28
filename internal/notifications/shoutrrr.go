package notifications

import (
	"context"

	"github.com/containrrr/shoutrrr"
)

type shoutrrrSender struct {
	url string
}

func (s *shoutrrrSender) Send(_ context.Context, message string) error {
	return shoutrrr.Send(s.url, message)
}
