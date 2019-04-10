package util

import (
	log "github.com/sirupsen/logrus"
	"sync"
)

type BlockingQueue interface {
	// Enqueue one item, block if the queue is full
	Put(item interface{})

	// Dequeue one item, block until it's available
	Take() interface{}

	// Dequeue one item, return nil if queue is empty
	Poll() interface{}

	// Return one item without dequeing, return nil if queue is empty
	Peek() interface{}

	// Return the current size of the queue
	Size() int

	// Return an iterator for the queue
	Iterator() BlockingQueueIterator
}

type BlockingQueueIterator interface {
	HasNext() bool
	Next() interface{}
}

type blockingQueue struct {
	items   []interface{}
	headIdx int
	tailIdx int
	size    int
	maxSize int

	mutex      sync.Mutex
	isNotEmpty *sync.Cond
	isNotFull  *sync.Cond
}

type blockingQueueIterator struct {
	bq      *blockingQueue
	readIdx int
	toRead  int
}

func NewBlockingQueue(maxSize int) BlockingQueue {
	bq := &blockingQueue{
		items:   make([]interface{}, maxSize),
		headIdx: 0,
		tailIdx: 0,
		size:    0,
		maxSize: maxSize,
	}

	bq.isNotEmpty = sync.NewCond(&bq.mutex)
	bq.isNotFull = sync.NewCond(&bq.mutex)
	return bq
}

func (bq *blockingQueue) Put(item interface{}) {
	bq.mutex.Lock()
	defer bq.mutex.Unlock()

	for ; bq.size == bq.maxSize; {
		bq.isNotFull.Wait()
	}

	wasEmpty := bq.size == 0

	bq.items[bq.tailIdx] = item
	bq.size += 1
	bq.tailIdx += 1
	if bq.tailIdx >= bq.maxSize {
		bq.tailIdx = 0
	}

	if wasEmpty {
		// Wake up eventual reader waiting for next item
		bq.isNotEmpty.Signal()
	}
}

func (bq *blockingQueue) Take() interface{} {
	bq.mutex.Lock()
	defer bq.mutex.Unlock()

	for ; bq.size == 0; {
		bq.isNotEmpty.Wait()
	}

	return bq.dequeue()
}

func (bq *blockingQueue) Poll() interface{} {
	bq.mutex.Lock()
	defer bq.mutex.Unlock()

	if bq.size == 0 {
		return nil
	}

	return bq.dequeue()
}

func (bq *blockingQueue) Peek() interface{} {
	bq.mutex.Lock()
	defer bq.mutex.Unlock()

	if bq.size == 0 {
		return nil
	} else {
		return bq.items[bq.headIdx]
	}
}

func (bq *blockingQueue) dequeue() interface{} {
	item := bq.items[bq.headIdx]
	bq.items[bq.headIdx] = nil

	bq.headIdx += 1
	if bq.headIdx == len(bq.items) {
		bq.headIdx = 0
	}

	bq.size -= 1
	bq.isNotFull.Signal()
	return item
}

func (bq *blockingQueue) Size() int {
	bq.mutex.Lock()
	defer bq.mutex.Unlock()

	return bq.size
}

func (bq *blockingQueue) Iterator() BlockingQueueIterator {
	bq.mutex.Lock()
	defer bq.mutex.Unlock()

	return &blockingQueueIterator{
		bq:      bq,
		readIdx: bq.headIdx,
		toRead:  bq.size,
	}
}

func (bqi *blockingQueueIterator) HasNext() bool {
	return bqi.toRead > 0
}

func (bqi *blockingQueueIterator) Next() interface{} {
	if bqi.toRead == 0 {
		log.Panic("Trying to read past the end of the iterator")
	}

	item := bqi.bq.items[bqi.readIdx]
	bqi.toRead--
	bqi.readIdx++
	if bqi.readIdx == bqi.bq.maxSize {
		bqi.readIdx = 0
	}
	return item
}
