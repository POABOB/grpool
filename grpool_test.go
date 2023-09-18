package grpool

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

const (
	KiB      uint64 = 1024
	MiB      uint64 = 1048576
	Param           = 100
	size            = 1000
	TestSize        = 10000
	n               = 100000
)

var curMem uint64

func demoPoolFunc(args interface{}) {
	n := args.(int)
	time.Sleep(time.Duration(n) * time.Millisecond)
}

func demoPoolFuncWithPanic() {
	panic("error")
}

// TestGrPoolWaitToGetWorker is used to test waiting to get worker.
func TestGrPoolWaitToGetWorker(t *testing.T) {
	var wg sync.WaitGroup
	p, _ := NewPool(size)
	defer p.Release()

	for i := 0; i < n; i++ {
		wg.Add(1)
		_ = p.Schedule(func() {
			demoPoolFunc(Param)
			wg.Done()
		})
	}
	wg.Wait()
	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestGrPoolWaitToGetWorkerPreMalloc(t *testing.T) {
	var wg sync.WaitGroup
	p, _ := NewPool(size, WithPreAlloc(true))
	defer p.Release()

	for i := 0; i < n; i++ {
		wg.Add(1)
		_ = p.Schedule(func() {
			demoPoolFunc(Param)
			wg.Done()
		})
	}
	wg.Wait()
	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

// TestGrPoolGetWorkerFromCache is used to test getting worker from sync.Pool.
func TestGrPoolGetWorkerFromCache(t *testing.T) {
	p, _ := NewPool(TestSize)
	defer p.Release()

	for i := 0; i < size; i++ {
		_ = p.Schedule(demoFunc)
	}
	time.Sleep(2 * DefaultCleanIntervalTime)
	_ = p.Schedule(demoFunc)
	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

// Contrast between goroutines without a pool and goroutines with ants pool.
func TestNoPool(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			demoFunc()
			wg.Done()
		}()
	}

	wg.Wait()
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

// TestGrPoolWithNewDefaultPool is used to test with the default settings.
func TestGrPoolWithNewDefaultPool(t *testing.T) {
	p := NewDefaultPool()
	defer p.Release()

	for i := 0; i < size; i++ {
		_ = p.Schedule(demoFunc)
	}
	time.Sleep(2 * DefaultCleanIntervalTime)
	_ = p.Schedule(demoFunc)
	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestGrPoolWithDefaultWorkersNumber(t *testing.T) {
	p, _ := NewPool(-1)

	for i := 0; i < size; i++ {
		_ = p.Schedule(demoFunc)
	}
	p.Release()
	p.Reboot()
	defer p.Release()
	for i := 0; i < size; i++ {
		_ = p.Schedule(demoFunc)
	}
	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestGrPoolWithPanicHandler(t *testing.T) {
	ph := func(v interface{}) {
		// dosomething...
		fmt.Printf("[panic occurred]: %v\n", v)
	}
	p, _ := NewPool(size, WithPanicHandler(ph))
	defer p.Release()

	for i := 0; i < 5; i++ {
		_ = p.Schedule(demoPoolFuncWithPanic)
	}

	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestGrPoolWithDisableClear(t *testing.T) {
	p, _ := NewPool(size, WithDisableClear(true))
	defer p.Release()

	for i := 0; i < size; i++ {
		_ = p.Schedule(demoFunc)
	}

	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestGrPoolWithNonblocking(t *testing.T) {
	p, _ := NewPool(size, WithNonblocking(true))
	defer p.Release()

	for i := 0; i < size; i++ {
		_ = p.Schedule(demoFunc)
	}

	t.Logf("pool, running workers number:%d", p.Running())
	t.Logf("pool, free workers number:%d", p.Free())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestGrPoolWithExpiryDuration(t *testing.T) {
	p, _ := NewPool(size, WithExpiryDuration(time.Second*5))
	defer p.Release()

	for i := 0; i < size; i++ {
		_ = p.Schedule(demoFunc)
	}

	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestGrPoolWithOptions(t *testing.T) {
	p, _ := NewPool(size, WithOptions(Options{Nonblocking: true}))
	defer p.Release()

	for i := 0; i < size; i++ {
		_ = p.Schedule(demoFunc)
	}

	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}
