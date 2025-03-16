package grpool

import (
	"sync"
	"testing"
	"time"
)

const (
	runTimes    = 1_000_000
	poolSize    = 100_000
	expiredTime = 10 * time.Second
)

func demoFunc() {
	time.Sleep(time.Duration(10) * time.Millisecond)
}

func BenchmarkGoroutines(b *testing.B) {
	var wg sync.WaitGroup
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(runTimes)
		for j := 0; j < runTimes; j++ {
			go func() {
				demoFunc()
				wg.Done()
			}()
		}
	}
	wg.Wait()
}

func BenchmarkGrpoolThroughput(b *testing.B) {
	var wg sync.WaitGroup
	p, _ := NewPool(poolSize, WithExpiryDuration(expiredTime), WithPreAlloc(true))
	defer p.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(runTimes)
		for j := 0; j < runTimes; j++ {
			_ = p.Schedule(demoFunc)
			wg.Done()
		}
	}
	wg.Wait()
}
