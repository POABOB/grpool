package grpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCircularQueue(t *testing.T) {
	size := 100
	q := newWorkerCircularQueue(size, false)
	assert.EqualValues(t, 0, q.len(), "Len error")
	assert.Equal(t, true, q.isEmpty(), "IsEmpty error")
	assert.Nil(t, q.detach(), "Dequeue error")
}

func TestCircularQueue(t *testing.T) {
	size := 10
	q := newWorkerCircularQueue(size, true)

	for i := 0; i < 5; i++ {
		err := q.insert(&Worker{lastUpdatedTime: time.Now()})
		if err != nil {
			break
		}
	}
	assert.EqualValues(t, 5, q.len(), "Len error")
	_ = q.detach()
	assert.EqualValues(t, 4, q.len(), "Len error")

	time.Sleep(time.Second)

	for i := 0; i < 6; i++ {
		err := q.insert(&Worker{lastUpdatedTime: time.Now()})
		if err != nil {
			break
		}
	}
	assert.EqualValues(t, 10, q.len(), "Len error")

	err := q.insert(&Worker{lastUpdatedTime: time.Now()})
	assert.Error(t, err, "Enqueue, error")

	q.refresh(time.Second)
	assert.EqualValuesf(t, 6, q.len(), "Len error: %d", q.len())
}

func TestRotatedQueueSearch(t *testing.T) {
	size := 10
	q := newWorkerCircularQueue(size, true)

	// 1
	expiry1 := time.Now()

	_ = q.insert(&Worker{lastUpdatedTime: time.Now()})

	assert.EqualValues(t, 0, q.binarySearch(time.Now()), "index should be 0")
	assert.EqualValues(t, -1, q.binarySearch(expiry1), "index should be -1")

	// 2
	expiry2 := time.Now()
	_ = q.insert(&Worker{lastUpdatedTime: time.Now()})

	assert.EqualValues(t, -1, q.binarySearch(expiry1), "index should be -1")

	assert.EqualValues(t, 0, q.binarySearch(expiry2), "index should be 0")

	assert.EqualValues(t, 1, q.binarySearch(time.Now()), "index should be 1")

	// more
	for i := 0; i < 5; i++ {
		_ = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}

	expiry3 := time.Now()
	_ = q.insert(&Worker{lastUpdatedTime: expiry3})

	var err error
	for err != errQueueIsFull {
		err = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}

	assert.EqualValues(t, 7, q.binarySearch(expiry3), "index should be 7")

	// rotate
	for i := 0; i < 6; i++ {
		_ = q.detach()
	}

	expiry4 := time.Now()
	_ = q.insert(&Worker{lastUpdatedTime: expiry4})

	for i := 0; i < 4; i++ {
		_ = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}
	//	head = 6, tail = 5, insert direction ->
	// [expiry4, time, time, time,  time, nil/tail,  time/head, time, time, time]
	assert.EqualValues(t, 0, q.binarySearch(expiry4), "index should be 0")

	for i := 0; i < 3; i++ {
		_ = q.detach()
	}
	expiry5 := time.Now()
	_ = q.insert(&Worker{lastUpdatedTime: expiry5})

	//	head = 6, tail = 5, insert direction ->
	// [expiry4, time, time, time,  time, expiry5,  nil/tail, nil, nil, time/head]
	assert.EqualValues(t, 5, q.binarySearch(expiry5), "index should be 5")

	for i := 0; i < 3; i++ {
		_ = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}
	//	head = 9, tail = 9, insert direction ->
	// [expiry4, time, time, time,  time, expiry5,  time, time, time, time/head/tail]
	assert.EqualValues(t, -1, q.binarySearch(expiry2), "index should be -1")

	assert.EqualValues(t, 9, q.binarySearch(q.items[9].getLastUpdatedTime()), "index should be 9")
	assert.EqualValues(t, 8, q.binarySearch(time.Now()), "index should be 8")
}

func TestRetrieveExpiry(t *testing.T) {
	size := 10
	q := newWorkerCircularQueue(size, true)
	expirew := make([]worker, 0)
	u, _ := time.ParseDuration("1s")

	// test [ time+1s, time+1s, time+1s, time+1s, time+1s, time, time, time, time, time]
	for i := 0; i < size/2; i++ {
		_ = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}
	expirew = append(expirew, q.items[:size/2]...)
	time.Sleep(u)

	for i := 0; i < size/2; i++ {
		_ = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}
	workers := q.refresh(u)

	assert.EqualValues(t, expirew, workers, "expired workers aren't right")

	// test [ time, time, time, time, time, time+1s, time+1s, time+1s, time+1s, time+1s]
	time.Sleep(u)

	for i := 0; i < size/2; i++ {
		_ = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}
	expirew = expirew[:0]
	expirew = append(expirew, q.items[size/2:]...)

	workers2 := q.refresh(u)

	assert.EqualValues(t, expirew, workers2, "expired workers aren't right")

	// test [ time+1s, time+1s, time+1s, nil, nil, time+1s, time+1s, time+1s, time+1s, time+1s]
	for i := 0; i < size/2; i++ {
		_ = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}
	for i := 0; i < size/2; i++ {
		_ = q.detach()
	}
	for i := 0; i < 3; i++ {
		_ = q.insert(&Worker{lastUpdatedTime: time.Now()})
	}
	time.Sleep(u)

	expirew = expirew[:0]
	expirew = append(expirew, q.items[0:3]...)
	expirew = append(expirew, q.items[size/2:]...)

	workers3 := q.refresh(u)

	assert.EqualValues(t, expirew, workers3, "expired workers aren't right")
}
