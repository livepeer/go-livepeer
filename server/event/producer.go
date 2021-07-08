package event

import (
	"context"
)

type Producer interface {
	Publish(ctx context.Context, key string, body interface{}) error
}
