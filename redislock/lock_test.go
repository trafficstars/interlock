package redislock

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/elliotchance/redismock"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
)

// newTestRedis returns a redis.Cmdable.
func newTestRedis(mr *miniredis.Miniredis) *redismock.ClientMock {
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return redismock.NewNiceMock(client)
}

func TestRedisLock(t *testing.T) {
	msg := struct{ s string }{s: `test`}
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()
	rlock := New(newTestRedis(mr), time.Minute)
	assert.NoError(t, rlock.TryLock(msg, time.Minute))
	assert.Error(t, rlock.TryLock(msg))
	assert.True(t, rlock.IsLocked(msg))
	assert.NoError(t, rlock.Unlock(msg))
	assert.NoError(t, rlock.TryLock(msg))
	assert.NoError(t, rlock.Expire(msg, time.Second))
	assert.Error(t, rlock.Expire(`void`, time.Second))
}

func TestRedisLockByURL(t *testing.T) {
	_, err := NewByURL(`redis://:password@localhost:6379/1`, time.Minute)
	assert.NoError(t, err)
	_, err = NewByURL(`x#:///%%@`, time.Minute)
	assert.Error(t, err)
}
