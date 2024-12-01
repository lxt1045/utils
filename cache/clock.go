package cache

import "github.com/benbjohnson/clock"

var sysClock = clock.New()

func SetClock(newClock clock.Clock) (oldClock clock.Clock) {
	oldClock = sysClock
	sysClock = newClock
	return oldClock
}
