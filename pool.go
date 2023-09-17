package grpool

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Pool struct {
	// pool 容量
	capacity int32

	// 正在執行的goroutine
	running int32

	// 閒置的Workers
	workers workerQueue

	// 警告Pool要自己close
	state int32

	// 鎖
	lock sync.Mutex

	// worker 等待的鎖
	cond *sync.Cond

	// 回收使用過的Worker
	workerCache sync.Pool

	// Clear 是否完成
	clearDone int32
	stopClear context.CancelFunc

	options *Options
}

// 初始化
func NewPool(size int, options ...Option) (*Pool, error) {

	// 加載設定
	opts := loadOptions(options...)

	if !opts.DisableClear {
		if expiry := opts.ExpiryDuration; expiry < 0 {
			return nil, ErrInvalidPoolExpiry
		} else if expiry == 0 {
			opts.ExpiryDuration = DefaultCleanIntervalTime
		}
	}

	// 如果 size 不是一個有效的 Size 就使用 DefaultPoolSize
	if size <= 0 {
		size = DefaultPoolSize
	}

	// init
	p := &Pool{
		capacity: int32(size),
		options:  opts,
	}

	p.workerCache.New = func() interface{} {
		return &Worker{
			pool: p,
			task: make(chan func(), workerChanCap),
		}
	}

	p.cond = sync.NewCond(&p.lock)

	p.workers = newWorkerCircularQueue(size, p.options.PreAlloc)

	// 定期清理過期的worker，節省系統資源
	p.goClear()

	return p, nil
}

// 開啟一個 goroutine 定時清理過期的 workers
func (p *Pool) goClear() {
	if p.options.DisableClear {
		return
	}

	var ctx context.Context
	ctx, p.stopClear = context.WithCancel(context.Background())
	go p.ClearStaleWorkers(ctx)
}

// 定時清理過期的 workers
func (p *Pool) ClearStaleWorkers(ctx context.Context) {
	ticker := time.NewTicker(p.options.ExpiryDuration)

	defer func() {
		ticker.Stop()
		atomic.StoreInt32(&p.clearDone, 1)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if p.IsClosed() {
				break
			}

			p.lock.Lock()
			staleWorkers := p.workers.refresh(p.options.ExpiryDuration)
			p.lock.Unlock()

			for i := range staleWorkers {
				staleWorkers[i].finish()
				staleWorkers[i] = nil
			}
		}
	}
}

// 獲取 worker 執行任務
func (p *Pool) Schedule(task func()) error {
	// 判斷Pool是否被關閉
	if p.IsClosed() {
		return ErrPoolClosed
	}

	if w := p.getWorker(); w != nil {
		w.inputFunc(task)
		return nil
	}
	return ErrPoolOverload
}

// 獲取 Pool 容量
func (p *Pool) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

// Free returns the number of available goroutines to work, -1 indicates this pool is unlimited.
func (p *Pool) Free() int {
	c := p.Cap()
	if c < 0 {
		return -1
	}
	return c - p.Running()
}

// 獲取正在執行的 Worker 數量
func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

func (p *Pool) addRunning(delta int) {
	atomic.AddInt32(&p.running, int32(delta))
}

// 清除 Pool 裡面的 Worker
func (p *Pool) Release() {
	if !atomic.CompareAndSwapInt32(&p.state, OPENED, CLOSED) {
		return
	}

	if p.stopClear != nil {
		p.stopClear()
		p.stopClear = nil
	}

	p.lock.Lock()
	p.workers.reset()
	p.lock.Unlock()

	p.cond.Broadcast()
}

// 重啟一個可以使用的 Pool
func (p *Pool) Reboot() {
	if atomic.CompareAndSwapInt32(&p.state, CLOSED, OPENED) {
		// 重新啟用 Worker Queue
		p.workers = newWorkerCircularQueue(int(p.capacity), p.options.PreAlloc)

		atomic.StoreInt32(&p.clearDone, 0)
		p.goClear()
	}
}

// 判斷是否被關閉
func (p *Pool) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == CLOSED
}

func (p *Pool) getWorker() (w worker) {
	genWorker := func() {
		if w = p.workerCache.Get().(*Worker); w == nil {
			// 新開一個worker
			w = &Worker{
				pool: p,
				task: make(chan func(), workerChanCap),
			}
		}
		w.run()
	}

	// 加鎖
	p.lock.Lock()
retry:
	if w = p.workers.detach(); w != nil {
		p.lock.Unlock()
		return
	}

	if cap := p.Cap(); cap > p.Running() {
		p.lock.Unlock()
		// 當前無可用worker，但是Pool沒有滿
		genWorker()
		return
	}

	if p.options.Nonblocking {
		p.lock.Unlock()
		return
	}

	// 阻塞等待
	p.cond.Wait()

	if p.IsClosed() {
		p.lock.Unlock()
		return
	}

	goto retry
}

// 將 Worker 放回 Pool
func (p *Pool) putWorker(worker *Worker) bool {
	// 避免 Worker 超出 Pool 容量，或是 Pool 已關閉
	cap := p.Cap()
	if cap > 0 && p.Running() > cap || p.IsClosed() {
		p.cond.Broadcast()
		return false
	}

	// 紀錄Woker最後一次運行時間
	worker.lastUpdatedTime = time.Now()

	p.lock.Lock()
	defer p.lock.Unlock()
	// 避免記憶體溢出
	if p.IsClosed() {
		return false
	}

	if err := p.workers.insert(worker); err != nil {
		return false
	}

	// 把 Blocking 等待 worker 的 task 喚醒
	p.cond.Signal()
	return true
}
