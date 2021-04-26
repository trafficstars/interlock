// Package interlock defines the interface for cross-service lock
package interlock

import "time"

// Locker provides simple inter-server locks
type Locker interface {
	TryLock(key interface{}, lifetime ...time.Duration) error
	Unlock(key interface{}) error
	IsLocked(key interface{}) bool
	Expire(key interface{}, lifetime ...time.Duration) error
}
