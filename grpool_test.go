package grpool

import (
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
