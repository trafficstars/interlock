package interlock

import "time"

type dummy struct{}

// NewDummy lock object
func NewDummy() Locker { return dummy{} }

func (d dummy) TryLock(key interface{}, lifetime ...time.Duration) error { return nil }
func (d dummy) Unlock(key interface{}) error                             { return nil }
func (d dummy) IsLocked(key interface{}) bool                            { return false }
