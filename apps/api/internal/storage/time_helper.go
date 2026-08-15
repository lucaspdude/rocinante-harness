package storage

import "time"

// nowUTC is a tiny helper for tests; separating it lets us swap
// for a clock in future phases without touching test bodies.
func nowUTC() time.Time { return time.Now().UTC() }
