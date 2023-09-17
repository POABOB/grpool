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
