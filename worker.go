package grpool

import (
	"fmt"
	"runtime/debug"
	"time"
)

type worker interface {
	run()
	finish()
	getLastUpdatedTime() time.Time
	inputFunc(func())
}

type Worker struct {
	// 任務池
	pool *Pool

	// 任務 func() error
	task chan func()

	// 回收時間
	lastUpdatedTime time.Time
}

func (w *Worker) run() {
	w.pool.addRunning(1)
	go func() {
		// 回收 Pool 失敗或 worker 發生錯誤
		defer func() {
			w.pool.addRunning(-1)
			// worker 放 cache 可以不用重新初始化
			w.pool.workerCache.Put(w)
			if p := recover(); p != nil {
				if ph := w.pool.options.PanicHandler; ph != nil {
					ph(p)
				} else {
					fmt.Printf("worker exited from panic: %v\n%s\n", p, debug.Stack())
				}
			}
			// 喚醒 Blocking 的 task
			w.pool.cond.Signal()
		}()

		// 監聽任務列表，有任務就拿出來執行
		for f := range w.task {
			// 被 finish()
			if f == nil {
				return
			}

			// 執行任務
			f()

			// 回收worker
			if ok := w.pool.putWorker(w); !ok {
				return
			}
		}
	}()
}

func (w *Worker) finish() {
	w.task <- nil
}

func (w *Worker) getLastUpdatedTime() time.Time {
	return w.lastUpdatedTime
}

func (w *Worker) inputFunc(fn func()) {
	w.task <- fn
}
