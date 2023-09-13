package gpool

import (
	"runtime/debug"
	"time"
)

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
		// worker panic 處理
		defer func() {
			w.pool.addRunning(-1)
			if p := recover(); p != nil {
				if ph := w.pool.options.PanicHandler; ph != nil {
					ph(p)
				} else {
					w.pool.options.Logger.Printf("worker exits from panic: %v\n%s\n", p, debug.Stack())
				}
			}
		}()

		// 監聽任務列表，有任務就拿出來執行
		for f := range w.task {
			if f == nil {
				return
			}
			// 執行任務
			f()

			// 回收worker
			if ok := w.pool.putBackWorker(w); !ok {
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
