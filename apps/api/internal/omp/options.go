package omp

import "context"

// optsContext is a tiny helper that wraps Options with a default
// background context for the factory seam.
func (o Options) optsContext() context.Context { return context.Background() }
