# Interlock

Package implements inter service lock based on different external storages.

## Example

```go
package main

import (
  "fmt"
  "log"
  "time"

  "github.com/demdxx/interlock/redislock"
  "github.com/demdxx/interlock"
)

func main() {
  const defaultLifetime = time.Minute
  rlock, err := redislock.NewByURL(`redis://host:3456/1?pool=10&max_retries=2&idle_cons=2`, defaultLifetime)
  if err != nil {
    log.Fatal(err)
  }

  if rlock.TryLock("start") {
    fmt.Println("I'm the first!")
  } else {
    fmt.Println("Someone ran first")
  }
}
```

## TODO

* [ ] Zookeeper
* [ ] Consul
* [ ] Aerospike
* [ ] Memcached
* [x] Redis
