package gpool

import "time"

// workerQueue 的實現
type loopQueue struct {
	items  []worker
	expiry []worker
	head   int
	tail   int
	size   int
	isFull bool
}

// 初始化 WorkerLoopQueue
func newWorkerLoopQueue(size int) *loopQueue {
	return &loopQueue{
		items: make([]worker, size),
		size:  size,
	}
}

// 獲取 Queue 長度，因為是 Loop Queue，所以會有 head 指標 > tail 指標的情況很正常
func (wq *loopQueue) len() int {
	// size == 0 || 頭尾一樣卻沒滿
	if wq.size == 0 || wq.isEmpty() {
		return 0
	}

	// 頭尾一樣滿了
	if wq.head == wq.tail && wq.isFull {
		return wq.size
	}

	if wq.tail > wq.head {
		return wq.tail - wq.head
	}

	return wq.size - wq.head + wq.tail
}

// 判斷 Queue 是否為空
func (wq *loopQueue) isEmpty() bool {
	return wq.head == wq.tail && !wq.isFull
}

// 插入一個 worker 進入 Queue
func (wq *loopQueue) insert(w worker) error {
	// Pool 已經被關閉
	if wq.size == 0 {
		return errQueueIsReleased
	}

	// Pool 已經滿了
	if wq.isFull {
		return errQueueIsFull
	}

	// 增加 Worker
	wq.items[wq.tail] = w
	wq.tail++

	// 如果 tail == size，讓 tail 變 0
	if wq.tail == wq.size {
		wq.tail = 0
	}
	// 如果 tail == head 代表滿了
	if wq.tail == wq.head {
		wq.isFull = true
	}

	return nil
}

// 從 Queue 獲取一個 worker
func (wq *loopQueue) detach() worker {
	if wq.isEmpty() {
		return nil
	}

	// 獲取一個 Worker
	w := wq.items[wq.head]
	wq.items[wq.head] = nil // 避免記憶體溢出
	wq.head++
	// 如果 head == size，讓 head 變 0
	if wq.head == wq.size {
		wq.head = 0
	}
	wq.isFull = false

	return w
}

// 重新整理 Queue，用於清理過期的 worker
func (wq *loopQueue) refresh(duration time.Duration) []worker {
	expiryTime := time.Now().Add(-duration)
	// 獲取過期 worker 的 index
	index := wq.binarySearch(expiryTime)
	if index == -1 {
		return nil
	}
	wq.expiry = wq.expiry[:0]

	if wq.head <= index {
		// 找出從"頭"到 "index" 的 worker，因為 FIFO 的關係，越前面的沒有被使用代表有一端時間未使用了
		wq.expiry = append(wq.expiry, wq.items[wq.head:index+1]...)
		for i := wq.head; i < index+1; i++ {
			wq.items[i] = nil
		}
	} else {
		// E.g. size = 10, tail = 5, head = 8 的狀態
		// 假設 index 是 3，那麼 8~9 和 0~3 的 worker 都要被清理，剩下 index 為 4, 5 的 worker
		wq.expiry = append(wq.expiry, wq.items[0:index+1]...)
		wq.expiry = append(wq.expiry, wq.items[wq.head:]...)
		for i := 0; i < index+1; i++ {
			wq.items[i] = nil
		}
		for i := wq.head; i < wq.size; i++ {
			wq.items[i] = nil
		}
	}

	// 清理過後，重新分配 head
	head := (index + 1) % wq.size
	wq.head = head
	if len(wq.expiry) > 0 {
		wq.isFull = false
	}

	// 返回這些過期 worker 要讓 pool 去手動 finish 它
	return wq.expiry
}

// 二元搜尋，找出過期的 worker
func (wq *loopQueue) binarySearch(expiryTime time.Time) int {
	var mid, nlen, basel, tmid int
	nlen = len(wq.items)

	// if no need to remove work, return -1
	if wq.isEmpty() || expiryTime.Before(wq.items[wq.head].getLastUpdatedTime()) {
		return -1
	}

	// example
	// size = 8, head = 7, tail = 4
	// [ 2, 3, 4, 5, nil, nil, nil,  1]  true position
	//   0  1  2  3    4   5     6   7
	//              tail          head
	//
	//   1  2  3  4  nil nil   nil   0   mapped position
	//            r                  l

	// base algorithm is a copy from worker_stack
	// map head and tail to effective left and right
	r := (wq.tail - 1 - wq.head + nlen) % nlen
	basel = wq.head
	l := 0
	for l <= r {
		mid = l + ((r - l) >> 1)
		// calculate true mid position from mapped mid position
		tmid = (mid + basel + nlen) % nlen
		if expiryTime.Before(wq.items[tmid].getLastUpdatedTime()) {
			r = mid - 1
		} else {
			l = mid + 1
		}
	}
	// return true position from mapped position
	return (r + basel + nlen) % nlen
}

//
func (wq *loopQueue) reset() {
	if wq.isEmpty() {
		return
	}

	// 逐一把
	for {
		if w := wq.detach(); w != nil {
			w.finish()
			continue
		}
		break
	}

	wq.items = wq.items[:0]
	wq.expiry = wq.expiry[:0]
	wq.size = 0
	wq.head = 0
	wq.tail = 0
	wq.isFull = false
}
