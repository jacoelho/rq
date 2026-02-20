package clock

import "time"

var nowFunc = time.Now

// Now returns the current time from the configured clock function.
func Now() time.Time {
	return nowFunc()
}

// SetNowForTest overrides the clock source and returns a restore function.
func SetNowForTest(fn func() time.Time) func() {
	previous := nowFunc
	nowFunc = fn
	return func() {
		nowFunc = previous
	}
}
