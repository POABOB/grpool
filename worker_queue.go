package gpool

import (
	"errors"
	"time"
)

var (
	// Queue 滿了
	errQueueIsFull = errors.New("the queue is full")

	// Queue 已經關閉了
	errQueueIsReleased = errors.New("the queue length is zero")
)

type worker interface {
	run()
	finish()
	getLastUpdatedTime() time.Time
	inputFunc(func())
}

type workerQueue interface {
	len() int
	isEmpty() bool
	insert(worker) error
	detach() worker
	refresh(duration time.Duration) []worker
	reset()
}

type queueType int

const (
	queueTypeStack     queueType = 1
	queueTypeLoopQueue queueType = 2
)

func newWorkerQueue(qType queueType, size int) workerQueue {
	switch qType {
	case queueTypeStack:
		return newWorkerStack(size)
	case queueTypeLoopQueue:
		return newWorkerLoopQueue(size)
	default:
		return newWorkerStack(size)
	}
}
