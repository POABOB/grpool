package gpool

import (
	"errors"
	"math"
	"runtime"
	"time"
)

const (
	// 預設 Pool 最大容量
	DefaultPoolSize = math.MaxInt32

	// 預設每 1 秒清理一次 Pool
	DefaultCleanIntervalTime = time.Second
)

// Pool 狀態
const (
	// Pool 開啟
	OPENED = 0

	// Pool 關閉
	CLOSED = 1
)

// 定義各種錯誤
var (
	ErrLackPoolFunc        = errors.New("must provide func for pool")
	ErrInvalidPoolSize     = errors.New("invalid pool size")
	ErrInvalidPoolExpiry   = errors.New("invalid pool expiry")
	ErrPoolClosed          = errors.New("pool has been closed")
	ErrPoolOverload        = errors.New("too many goroutines blocked or Nonblocking is set")
	ErrInvalidPreAllocSize = errors.New("can not set up a negative capacity under PreAlloc mode")
	ErrTimeout             = errors.New("operation timed out")

	// workerChanCap determines whether the channel of a worker should be a buffered channel
	// to get the best performance. Inspired by fasthttp at
	// https://github.com/valyala/fasthttp/blob/master/workerpool.go#L139
	workerChanCap = func() int {
		// Use blocking channel if GOMAXPROCS=1.
		// This switches context from sender to receiver immediately,
		// which results in higher performance (under go1.5 at least).
		if runtime.GOMAXPROCS(0) == 1 {
			return 0
		}

		// Use non-blocking workerChan if GOMAXPROCS>1,
		// since otherwise the sender might be dragged down if the receiver is CPU-bound.
		return 1
	}()

	// 預設的 Pool
	defaultPool *Pool
)

const nowTimeUpdateInterval = 500 * time.Millisecond

// Logger is used for logging formatted messages.
type Logger interface {
	// Printf must have the same semantics as log.Printf.
	Printf(format string, args ...interface{})
}

func NewDefaultPool() (defaultPool *Pool, err error) {
	// 初始化一個預設的 Pool
	defaultPool, err = NewPool(DefaultPoolSize)
	return
}

// 提交任務至 Goroutine Pool
func Schedule(task func()) error {
	return defaultPool.Schedule(task)
}

// 獲取目前正在執行的 Worker 數量
func Running() int {
	return defaultPool.Running()
}

// 獲取目前 Pool 的最大容量
func Cap() int {
	return defaultPool.Cap()
}

// 獲取有空的 Worker 數量
func Free() int {
	return defaultPool.Free()
}

// 關閉 Pool
func Release() {
	defaultPool.Release()
}

// 關閉 Pool 但是會有一個超時限制去等待 Workers 都關閉
func ReleaseWithTimeout(timeout time.Duration) error {
	return defaultPool.ReleaseWithTimeout(timeout)
}

// 重啟 Pool
func Reboot() {
	defaultPool.Reboot()
}
