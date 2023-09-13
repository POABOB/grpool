package grpool

import "time"

// workerQueue 的實現 使用 stack 方式
type workerStack struct {
	items  []worker
	expiry []worker
}

// 初始化 newWorkerStack
func newWorkerStack(size int) *workerStack {
	return &workerStack{
		items: make([]worker, 0, size),
	}
}

// 獲取 Queue 長度
func (wq *workerStack) len() int {
	return len(wq.items)
}

// 判斷 Queue 是否為空
func (wq *workerStack) isEmpty() bool {
	return len(wq.items) == 0
}

// 插入一個 worker 進入 Queue
func (wq *workerStack) insert(w worker) error {
	wq.items = append(wq.items, w)
	return nil
}

// 從 Queue 獲取一個 worker
func (wq *workerStack) detach() worker {
	l := wq.len()
	if l == 0 {
		return nil
	}

	w := wq.items[l-1]
	wq.items[l-1] = nil // 必免記憶體溢出
	wq.items = wq.items[:l-1]

	return w
}

// 重新整理 Queue，用於清理過期的 worker
func (wq *workerStack) refresh(duration time.Duration) []worker {
	n := wq.len()
	if n == 0 {
		return nil
	}

	expiryTime := time.Now().Add(-duration)
	index := wq.binarySearch(0, n-1, expiryTime)

	wq.expiry = wq.expiry[:0]
	if index != -1 {
		wq.expiry = append(wq.expiry, wq.items[:index+1]...)
		m := copy(wq.items, wq.items[index+1:])
		for i := m; i < n; i++ {
			wq.items[i] = nil
		}
		wq.items = wq.items[:m]
	}
	return wq.expiry
}

// 二元搜尋，找出過期的 worker
func (wq *workerStack) binarySearch(l, r int, expiryTime time.Time) int {
	for l <= r {
		mid := int(uint(l+r) >> 1) // avoid overflow when computing mid
		if expiryTime.Before(wq.items[mid].getLastUpdatedTime()) {
			r = mid - 1
		} else {
			l = mid + 1
		}
	}
	return r
}

// 當 Pool 被 Release 後，就會觸發此方法，將所有 Worker queue 清理
func (wq *workerStack) reset() {
	for i := 0; i < wq.len(); i++ {
		wq.items[i].finish()
		wq.items[i] = nil
	}
	wq.items = wq.items[:0]
	wq.expiry = wq.expiry[:0]
}
