package grpool

import (
	"context"
	"log"
	"os"
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

	// Blocking 的 task 數
	waiting int32

	// Purge 是否完成
	purgeDone int32
	stopPurge context.CancelFunc

	// 定時清理是否完成
	heartBeatDone int32
	stopHeartBeat context.CancelFunc

	now atomic.Value

	options *Options
}

// 初始化
func NewPool(size int, options ...Option) (*Pool, error) {

	// 不確定要用 Stack 還是 Queue，先定義 -1 讓它能夠無限制增長
	if size <= 0 {
		size = -1
	}

	opts := loadOptions(options...)

	if !opts.DisablePurge {
		if expiry := opts.ExpiryDuration; expiry < 0 {
			return nil, ErrInvalidPoolExpiry
		} else if expiry == 0 {
			opts.ExpiryDuration = DefaultCleanIntervalTime
		}
	}

	// 如果沒有自訂義Logger
	if opts.Logger == nil {
		opts.Logger = Logger(log.New(os.Stderr, "[Error]: ", log.LstdFlags|log.Lmsgprefix|log.Lmicroseconds))
	}

	// init
	p := &Pool{
		capacity: int32(size),
		options:  opts,
	}

	if p.options.PreAlloc {
		if size == -1 {
			return nil, ErrInvalidPreAllocSize
		}
		// Loop Queue
		p.workers = newWorkerQueue(queueTypeLoopQueue, size)
	} else {
		// Stack
		p.workers = newWorkerQueue(queueTypeStack, 0)
	}

	// 定期清理過期的worker，節省系統資源
	p.goPurge()
	p.goHeartBeat()

	return p, nil
}

// 開啟一個 goroutine 定時清理過期的 workers
func (p *Pool) goPurge() {
	if p.options.DisablePurge {
		return
	}

	var ctx context.Context
	ctx, p.stopPurge = context.WithCancel(context.Background())
	go p.purgeStaleWorkers(ctx)
}

// 定時清理過期的 workers
func (p *Pool) purgeStaleWorkers(ctx context.Context) {
	ticker := time.NewTicker(p.options.ExpiryDuration)

	defer func() {
		ticker.Stop()
		atomic.StoreInt32(&p.purgeDone, 1)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if p.IsClosed() {
			break
		}

		p.lock.Lock()
		staleWorkers := p.workers.refresh(p.options.ExpiryDuration)
		p.lock.Unlock()

		// Notify obsolete workers to stop.
		// This notification must be outside the p.lock, since w.task
		// may be blocking and may consume a lot of time if many workers
		// are located on non-local CPUs.
		for i := range staleWorkers {
			staleWorkers[i].finish()
			staleWorkers[i] = nil
		}
	}
}

// 開啟一個 goroutine 定時更新 Pool 的時間
func (p *Pool) goHeartBeat() {
	p.now.Store(time.Now())
	var ctx context.Context
	ctx, p.stopHeartBeat = context.WithCancel(context.Background())
	go p.heartBeat(ctx)
}

// 定時更新 Pool 的時間
func (p *Pool) heartBeat(ctx context.Context) {
	ticker := time.NewTicker(nowTimeUpdateInterval)
	defer func() {
		ticker.Stop()
		atomic.StoreInt32(&p.heartBeatDone, 1)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if p.IsClosed() {
			break
		}

		p.now.Store(time.Now())
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

// Waiting returns the number of tasks which are waiting be executed.
func (p *Pool) Waiting() int {
	return int(atomic.LoadInt32(&p.waiting))
}

// 獲取正在執行的 Worker 數量
func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

func (p *Pool) addWaiting(delta int) {
	atomic.AddInt32(&p.waiting, int32(delta))
}

func (p *Pool) addRunning(delta int) {
	atomic.AddInt32(&p.running, int32(delta))
}

// 清除 Pool 裡面的 Worker
func (p *Pool) Release() {
	if !atomic.CompareAndSwapInt32(&p.state, OPENED, CLOSED) {
		return
	}

	if p.stopPurge != nil {
		p.stopPurge()
		p.stopPurge = nil
	}
	p.stopHeartBeat()
	p.stopHeartBeat = nil

	p.lock.Lock()
	p.workers.reset()
	p.lock.Unlock()
}

// 在一定時間內清除 Pool 裡面的 Worker
func (p *Pool) ReleaseWithTimeout(timeout time.Duration) error {
	if p.IsClosed() || (!p.options.DisablePurge && p.stopPurge == nil) || p.stopHeartBeat == nil {
		return ErrPoolClosed
	}
	p.Release()

	endTime := time.Now().Add(timeout)
	for time.Now().Before(endTime) {
		if p.Running() == 0 &&
			(p.options.DisablePurge || atomic.LoadInt32(&p.purgeDone) == 1) &&
			atomic.LoadInt32(&p.heartBeatDone) == 1 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return ErrTimeout
}

// 重啟一個可以使用的 Pool
func (p *Pool) Reboot() {
	if atomic.CompareAndSwapInt32(&p.state, CLOSED, OPENED) {

		// 重新啟用 Worker Queue
		if p.options.PreAlloc {
			p.workers = newWorkerQueue(queueTypeLoopQueue, int(p.capacity))
		} else {
			p.workers = newWorkerQueue(queueTypeStack, 0)
		}

		atomic.StoreInt32(&p.purgeDone, 0)
		p.goPurge()
		atomic.StoreInt32(&p.heartBeatDone, 0)
		p.goHeartBeat()
	}
}

// 判斷是否被關閉
func (p *Pool) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == CLOSED
}

func (p *Pool) getWorker() (w worker) {
	spawnWorker := func() {
		// 新開一個worker
		w = &Worker{
			pool: p,
			task: make(chan func(), workerChanCap),
		}
		w.run()
	}

	// 加鎖
	p.lock.Lock()
	w = p.workers.detach()

	if w != nil { // 直接獲取到 worker 執行任務
		p.lock.Unlock()
	} else if cap := p.Cap(); cap == -1 || cap > p.Running() {
		// if the worker queue is empty and we don't run out of the pool capacity,
		// then just spawn a new worker goroutine.
		p.lock.Unlock()
		// 當前無可用worker，但是Pool沒有滿
		spawnWorker()
	} else { // otherwise, we'll have to keep them blocked and wait for at least one worker to be put back into pool.
		if p.options.Nonblocking {
			p.lock.Unlock()
			return
		}
		// 阻塞等待，等到有可用worker
		p.addWaiting(1)
		for {
			// 避免 blocking tasks 超出最大值
			if p.options.MaxBlockingTasks != 0 && p.Waiting() >= p.options.MaxBlockingTasks {
				p.lock.Unlock()
				return
			}

			if p.IsClosed() {
				p.addWaiting(-1)
				p.lock.Unlock()
				return
			}

			if w = p.workers.detach(); w == nil {
				if p.Free() > 0 {
					p.addWaiting(-1)
					p.lock.Unlock()
					spawnWorker()
					return
				}
				continue
			}
			p.addWaiting(-1)
			p.lock.Unlock()
		}
	}

	return
}

// 將 Worker 放回 Pool
func (p *Pool) putBackWorker(worker *Worker) bool {
	// 避免 Worker 超出 Pool 容量，或是 Pool 已關閉
	cap := p.Cap()
	if (cap > 0 && p.Running() > cap) || p.IsClosed() {
		return false
	}

	// 紀錄Woker最後一次運行時間
	worker.lastUpdatedTime = time.Now()

	p.lock.Lock()
	// 避免記憶體溢出
	// Issue: https://github.com/panjf2000/ants/issues/113
	if p.IsClosed() {
		p.lock.Unlock()
		return false
	}

	if err := p.workers.insert(worker); err != nil {
		p.lock.Unlock()
		return false
	}

	p.lock.Unlock()
	return true
}
