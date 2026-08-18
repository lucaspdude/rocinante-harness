package files

import (
	"context"
	"time"
)

// newProcTimeout returns a context with a 5s deadline for git
// shellouts. Pluggable so tests can substitute.
var newProcTimeout = func() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
