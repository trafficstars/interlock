// Package redislock implements locker based on Redis server
package redislock

import (
	"errors"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/demdxx/gocast"
	"github.com/go-redis/redis"
)

var (
	errLockHasFailed    = errors.New(`redis lock has failed`)
	errLockDoesNotExist = errors.New(`redis lock for provided key does not exist`)
)

const maxSlotsCount = 16384

// Lock provides redis key locker
type Lock struct {
	lifetime time.Duration

	activeClient redis.Cmdable
	clientPool   []redis.Cmdable

	mtx sync.Mutex
}

// New returns redis Lock for redis client
func New(client redis.Cmdable, defaultLifetime time.Duration) *Lock {
	return &Lock{activeClient: client, lifetime: defaultLifetime}
}

// NewByURL returns redis Lock object or error
// Example "redis://host1:6379,host2:6379/12"
func NewByURL(connectURL string, defaultLifetime time.Duration) (*Lock, error) {
	var (
		connectURLObj, err = url.Parse(connectURL)
		password           string
	)
	if err != nil {
		return nil, err
	}
	if connectURLObj.User != nil {
		password, _ = connectURLObj.User.Password()
	}
	hosts := strings.Split(connectURLObj.Host, ",")

	// the first node will be the master
	clientPool := make([]redis.Cmdable, 0)
	for _, addr := range hosts {
		clientPool = append(clientPool, redis.NewClient(&redis.Options{
			DB:           gocast.ToInt(strings.Trim(connectURLObj.Path, `/`)),
			Addr:         addr,
			Password:     password,
			PoolSize:     gocast.ToInt(connectURLObj.Query().Get(`pool`)),
			MaxRetries:   gocast.ToInt(connectURLObj.Query().Get(`max_retries`)),
			MinIdleConns: gocast.ToInt(connectURLObj.Query().Get(`idle_cons`)),
		}))
	}

	return &Lock{
		lifetime:     defaultLifetime,
		activeClient: clientPool[0],
		clientPool:   clientPool,
		mtx:          sync.Mutex{},
	}, nil
}

// TryLock message as processing
func (mr *Lock) TryLock(key interface{}, lifetime ...time.Duration) error {
	var (
		err error
		res bool
	)
	lt := mr.lifetime
	if len(lifetime) == 1 {
		lt = lifetime[0]
	}
	mr.mtx.Lock()
	for attempts := 0; attempts < len(mr.clientPool); attempts++ {
		res, err = mr.activeClient.SetNX(hash(key), []byte(`t`), lt).Result()
		if isNetworkError(err) {
			mr.refreshActiveClient()
			continue
		}
		if err == nil && !res {
			err = errLockHasFailed
		}
		mr.mtx.Unlock()
		return err
	}
	mr.mtx.Unlock()
	return err
}

func (mr *Lock) refreshActiveClient() {
	if len(mr.clientPool) > 1 {
		mr.clientPool = mr.clientPool[1:]
		mr.clientPool = append(mr.clientPool, mr.activeClient)
		mr.activeClient = mr.clientPool[0]
	}
}

// IsLocked in the redis server
func (mr *Lock) IsLocked(key interface{}) bool {
	var (
		err error
		val string
	)
	mr.mtx.Lock()
	for attempts := 0; attempts < len(mr.clientPool); attempts++ {
		val, err = mr.activeClient.Get(hash(key)).Result()
		if isNetworkError(err) {
			mr.refreshActiveClient()
			continue
		}
		mr.mtx.Unlock()
		return val == `t`
	}
	mr.mtx.Unlock()
	return false
}

// Unlock message as processing
func (mr *Lock) Unlock(key interface{}) error {
	var err error
	mr.mtx.Lock()
	for attempts := 0; attempts < len(mr.clientPool); attempts++ {
		err = mr.activeClient.Del(hash(key)).Err()
		if isNetworkError(err) {
			mr.refreshActiveClient()
			continue
		}
		mr.mtx.Unlock()
		return err
	}
	mr.mtx.Unlock()
	return err
}

// Expire TTL of existing lock
func (mr *Lock) Expire(key interface{}, lifetime ...time.Duration) error {
	var (
		err error
		res bool
	)
	lt := mr.lifetime
	if len(lifetime) == 1 {
		lt = lifetime[0]
	}
	mr.mtx.Lock()
	for attempts := 0; attempts < len(mr.clientPool); attempts++ {
		res, err = mr.activeClient.Expire(hash(key), lt).Result()
		if isNetworkError(err) {
			mr.refreshActiveClient()
			continue
		}
		if err == nil && !res {
			err = errLockDoesNotExist
		}
		mr.mtx.Unlock()
		return err
	}
	mr.mtx.Unlock()
	return err
}

func isNetworkError(err error) bool {
	if err == io.EOF {
		return true
	}
	cause := err
	for {
		if unwrap, ok := cause.(interface{ Unwrap() error }); ok {
			cause = unwrap.Unwrap()
			continue
		}
		break
	}

	if cause, ok := cause.(*net.DNSError); ok && cause.Err == "no such host" {
		return true
	}

	if cause, ok := cause.(syscall.Errno); ok {
		if cause == 10061 || cause == syscall.ECONNREFUSED {
			return true
		}
	}

	if _, ok := cause.(net.Error); ok {
		return true
	}

	return false
}
