// Package redislock implements locker based on Redis server
package redislock

import (
	"errors"
	"net/url"
	"sort"
	"strings"
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
	client   redis.Cmdable
	Cluster  *redis.ClusterClient
}

// New returns redis Lock for redis client
func New(client redis.Cmdable, defaultLifetime time.Duration) *Lock {
	return &Lock{client: client, lifetime: defaultLifetime}
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
	if len(hosts) > 1 {

		// the first node will be the master
		//nodes := make([]redis.ClusterNode, 0)
		//for _, addr := range hosts {
		//	nodes = append(nodes, redis.ClusterNode{Addr: addr})
		//}

		rdb := redis.NewClusterClient(&redis.ClusterOptions{
			//ClusterSlots: func() ([]redis.ClusterSlot, error) {
			//	return []redis.ClusterSlot{{
			//		Start: 0,
			//		End:   maxSlotsCount,
			//		Nodes: nodes,
			//	}}, nil
			//},

			ClusterSlots: func() ([]redis.ClusterSlot, error) {
				nodes := make([]redis.ClusterNode, 0)
				latency := make([]int64, 0)
				avalibleHosts := make([]string, 0)
				for _, host := range hosts {
					c := redis.NewClient(&redis.Options{Addr: host})
					start := time.Now()
					c.Ping()
					late := time.Since(start)
					if late.Seconds() > time.Second.Seconds() {
						continue
					}
					latency = append(latency, late.Nanoseconds())
					avalibleHosts = append(avalibleHosts, host)
					c.Close()
				}
				sort.SliceIsSorted(avalibleHosts, func(i, j int) bool {
					return latency[i] < latency[j]
				})
				for _, addr := range avalibleHosts {
					nodes = append(nodes, redis.ClusterNode{Addr: addr})
				}
				return []redis.ClusterSlot{redis.ClusterSlot{
					Start: 0,
					End:   maxSlotsCount,
					Nodes: nodes,
				}}, nil
			},
		})
		return New(rdb, defaultLifetime), nil
	}
	return New(redis.NewClient(&redis.Options{
		DB:           gocast.ToInt(strings.Trim(connectURLObj.Path, `/`)),
		Addr:         connectURLObj.Host,
		Password:     password,
		PoolSize:     gocast.ToInt(connectURLObj.Query().Get(`pool`)),
		MaxRetries:   gocast.ToInt(connectURLObj.Query().Get(`max_retries`)),
		MinIdleConns: gocast.ToInt(connectURLObj.Query().Get(`idle_cons`)),
	}), defaultLifetime), nil
}

// TryLock message as processing
func (mr *Lock) TryLock(key interface{}, lifetime ...time.Duration) error {
	lt := mr.lifetime
	if len(lifetime) == 1 {
		lt = lifetime[0]
	}
	res, err := mr.client.SetNX(hash(key), []byte(`t`), lt).Result()
	if err == nil && !res {
		err = errLockHasFailed
	}

	return err
}

// IsLocked in the redis server
func (mr *Lock) IsLocked(key interface{}) bool {
	val, _ := mr.client.Get(hash(key)).Result()
	return val == `t`
}

// Unlock message as processing
func (mr *Lock) Unlock(key interface{}) error {
	return mr.client.Del(hash(key)).Err()
}

// Expire TTL of existing lock
func (mr *Lock) Expire(key interface{}, lifetime ...time.Duration) error {
	lt := mr.lifetime
	if len(lifetime) == 1 {
		lt = lifetime[0]
	}
	res, err := mr.client.Expire(hash(key), lt).Result()
	if err == nil && !res {
		err = errLockDoesNotExist
	}
	return err
}
