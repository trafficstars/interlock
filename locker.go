// Package interlock defines the interface for cross-service lock
package interlock

import "time"

// Locker provides simple interserver locks
type Locker interface {
	TryLock(key interface{}, lifetime ...time.Duration) error
	Unlock(key interface{}) error
	IsLocked(key interface{}) bool
}
