package eth

import (
	"context"
)

type Service interface {
	Start(context.Context) error
	Stop() error
}
