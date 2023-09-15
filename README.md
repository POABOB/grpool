# grpool

<a title="Build Status" target="_blank" href="https://github.com/POABOB/grpool/actions?query=workflow%3AGo"><img src="https://img.shields.io/github/actions/workflow/status/POABOB/grpool/go.yaml?branch=main&style=flat-square&logo=github-actions" /></a>
<a title="Release" target="_blank" href="https://github.com/POABOB/grpool/releases"><img src="https://img.shields.io/github/v/release/POABOB/grpool.svg?color=166823&style=flat-square&logo=smartthings" /></a>
<a title="Go Report Card" target="_blank" href="https://goreportcard.com/report/github.com/POABOB/grpool"><img src="https://goreportcard.com/badge/github.com/POABOB/grpool?style=flat-square" /></a>


## Introduction

`grpool` is a groutine pool which can provide a fixed size of capacity, recycle the stale workers. 

## Installation

```bash
go get -u github.com/POABOB/grpool
```

## How to use

If you want to process a massive number of jobs, you don't need to use the same number of goroutines. It's wasteful to use excessive memory, leading to high consumption.

### A Simple Example with Default Pool

```go
package main

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/POABOB/grpool"
)

var wg sync.WaitGroup
var count int32 = 0

func main() {
	// Use default Pool Will use the math.MaxInt32 of capacity.
	pool := grpool.NewDefaultPool()
	// Release worker resource
	defer pool.Release()

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		_ = pool.Schedule(func() {
			printFunc(i)
		})
	}

	wg.Wait()
    
	fmt.Printf("running goroutines: %d\n", pool.Running())
}

func printFunc(i int) {
	atomic.AddInt32(&count, 1)
	fmt.Println("Hello World! I am worker" + strconv.Itoa(i) + " from goroutine pool.")
	wg.Done()
}
```

### Limit the goroutines and pre-alloc it

```go
package main

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/POABOB/grpool"
)

var count int32 = 0
var wg sync.WaitGroup

func main() {
	// init a capacity of 18 goroutine pool with preallocing the space.
	pool, _ := grpool.NewPool(18, grpool.WithPreAlloc(true))
	// Release worker resource
	defer pool.Release()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		_ = pool.Schedule(func() {
			printFunc(i)
		})
	}

	wg.Wait()

	fmt.Printf("running goroutines: %d\n", pool.Running())
}

func printFunc(i int) {
	atomic.AddInt32(&count, 1)
	fmt.Println("Hello World! I am worker" + strconv.Itoa(i) + " from goroutine pool.")
	wg.Done()
}
```

### Use non-blocking pool

```go
pool, err := grpool.NewPool(1000, grpool.WithNonblocking(true))
```


### Customize panic handler

```go
func ph(v interface{}) {
    // dosomething...
	fmt.Printf("[panic occurred]: %v\n", v)
}
pool, err := grpool.NewPool(1000, grpool.WithPanicHandler(ph))
```

### Customize the time interval of clear stale worker

```go
pool, err := grpool.NewPool(1000, grpool.WithExpiryDuration(time.Second * 5))
```


## License

[MIT License](https://github.com/POABOB/grpool/blob/main/LICENSE)