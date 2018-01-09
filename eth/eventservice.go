package eth

import (
	"context"
)

type EventService interface {
	Start(context.Context) error
	Stop() error
}
