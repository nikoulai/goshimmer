package ratecontrol

import (
	"bytes"
	"container/heap"
	"math/big"
	"sync"

	"github.com/iotaledger/hive.go/bitmask"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region MessageQueue /////////////////////////////////////////////////////////////////////////////////////////////////

type MessageQueue struct {
	messageHeap      messageHeap
	messageHeapMutex sync.RWMutex

	waitCond *sync.Cond

	maxSize int

	shutdownSignal chan byte
	isShutdown     bool
	shutdownFlags  ShutdownFlag
	shutdownMutex  sync.Mutex
}

// NewMessageQueue the constructor for the MessageQueue.
func NewMessageQueue(opts ...Option) (queue *MessageQueue) {
	queue = &MessageQueue{
		shutdownSignal: make(chan byte),
	}
	queue.waitCond = sync.NewCond(&queue.messageHeapMutex)

	for _, opt := range opts {
		opt(queue)
	}

	return
}

func (m *MessageQueue) Queue(message *Message) (addedElement *QueuedMessage) {
	// sanitize parameters
	if message == nil {
		panic("<nil> must not be added to the queue")
	}

	// prevent modifications of a shutdown queue
	if m.IsShutdown() {
		if m.shutdownFlags.HasBits(PanicOnModificationsAfterShutdown) {
			panic("tried to modify a shutdown TimedQueue")
		}

		return
	}

	// acquire locks
	m.messageHeapMutex.Lock()

	// add new element
	addedElement = &QueuedMessage{
		messageQueue: m,
		ID:           message.ID(),
		Priority:     message.Priority,
		Parents:      message.ParentsByType(tangle.StrongParentType),
		cancel:       make(chan byte),
		index:        0,
	}
	heap.Push(&m.messageHeap, addedElement)

	if m.maxSize > 0 {
		// heap is bigger than maxSize now; remove the last element (furthest in the future).
		if size := m.messageHeap.Len(); size > m.maxSize {
			heap.Remove(&m.messageHeap, 0)
		}
	}

	// release locks
	m.messageHeapMutex.Unlock()

	// signal waiting goroutine to wake up
	m.waitCond.Signal()

	return
}

// Poll returns the first value of this queue. It waits for the scheduled time before returning and is therefore
// blocking. It returns nil if the queue is empty.
func (t *MessageQueue) Poll(waitIfEmpty bool) interface{} {
	for {
		// acquire locks
		t.messageHeapMutex.Lock()

		// wait for elements to be queued
		for len(t.messageHeap) == 0 {
			if !waitIfEmpty || t.IsShutdown() {
				t.messageHeapMutex.Unlock()
				return nil
			}

			t.waitCond.Wait()
		}

		// retrieve first element
		polledElement := heap.Remove(&t.messageHeap, len(t.messageHeap) - 1).(*QueuedMessage)

		// release locks
		t.messageHeapMutex.Unlock()

		// wait for the return value to become due
		select {
		// react if the queue was shutdown while waiting
		case <-t.shutdownSignal:
			// abort if the pending elements are supposed to be canceled
			if t.shutdownFlags.HasBits(CancelPendingElements) {
				return nil
			}

			// wait for the return value to become due
			select {
			// abort waiting for this element and return the next one instead if it was canceled
			case <-polledElement.cancel:
				continue

			// return the result after the time is reached
			default:
				return polledElement.ID
			}

		// abort waiting for this element and return the next one instead if it was canceled
		case <-polledElement.cancel:
			continue

		// return the result after the time is reached
		default:
			return polledElement.ID
		}
	}
}

// IsShutdown returns true if this queue was shutdown.
func (m *MessageQueue) IsShutdown() bool {
	m.shutdownMutex.Lock()
	defer m.shutdownMutex.Unlock()

	return m.isShutdown
}

// Size returns the amount of elements that are currently enqueued in this queue.
func (m *MessageQueue) Size() int {
	m.messageHeapMutex.RLock()
	defer m.messageHeapMutex.RUnlock()

	return len(m.messageHeap)
}

// Shutdown terminates the queue. It accepts an optional list of shutdown flags that allows the caller to modify the
// shutdown behavior.
func (m *MessageQueue) Shutdown(optionalShutdownFlags ...ShutdownFlag) {
	// acquire locks
	m.shutdownMutex.Lock()

	// prevent modifications of an already shutdown queue
	if m.isShutdown {
		// automatically unlock
		defer m.shutdownMutex.Unlock()

		// panic if the corresponding flag was set
		if m.shutdownFlags.HasBits(PanicOnModificationsAfterShutdown) {
			panic("tried to shutdown and already shutdown TimedQueue")
		}

		return
	}

	// mark the queue as shutdown
	m.isShutdown = true

	// store the shutdown flags
	for _, shutdownFlag := range optionalShutdownFlags {
		m.shutdownFlags |= shutdownFlag
	}

	// release the lock
	m.shutdownMutex.Unlock()

	// close the shutdown channel (notify waiting threads)
	close(m.shutdownSignal)

	m.messageHeapMutex.Lock()
	switch queuedElementsCount := len(m.messageHeap); queuedElementsCount {
	// if the queue is empty ...
	case 0:
		// ... stop waiting for new elements
		m.waitCond.Broadcast()

	// if the queue is not empty ...
	default:
		// ... empty it if the corresponding flag was set
		if m.shutdownFlags.HasBits(CancelPendingElements) {
			for i := 0; i < queuedElementsCount; i++ {
				heap.Remove(&m.messageHeap, 0)
			}
		}
	}
	m.messageHeapMutex.Unlock()
}

// removeElement is an internal utility function that removes the given element from the queue.
func (m *MessageQueue) removeElement(element *QueuedMessage) {
	// abort if the element was removed already
	if element.index == -1 {
		return
	}

	// remove the element
	heap.Remove(&m.messageHeap, element.index)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region QueuedMessage ////////////////////////////////////////////////////////////////////////////////////////////////

type QueuedMessage struct {
	ID       tangle.MessageID
	Priority uint64
	Parents  tangle.MessageIDs

	messageQueue *MessageQueue
	cancel     chan byte
	index      int
}

// Cancel removed the given element from the queue and cancels its execution.
func (timedQueueElement *QueuedMessage) Cancel() {
	// acquire locks
	timedQueueElement.messageQueue.messageHeapMutex.Lock()
	defer timedQueueElement.messageQueue.messageHeapMutex.Unlock()

	// remove element from queue
	timedQueueElement.messageQueue.removeElement(timedQueueElement)

	// close the cancel channel to notify subscribers
	close(timedQueueElement.cancel)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Message //////////////////////////////////////////////////////////////////////////////////////////////////////

type Message struct {
	*tangle.Message

	Priority uint64
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region messageHeap //////////////////////////////////////////////////////////////////////////////////////////////////

type messageHeap []*QueuedMessage

// Len is the number of elements in the collection.
func (h messageHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i should sort before the element with index j.
func (h messageHeap) Less(i, j int) bool {
	if h[i].Priority < h[j].Priority || h.leadingZeros(h[i].ID.Bytes()) < h.leadingZeros(h[j].ID.Bytes()) {
		return true
	}

	return bytes.Compare(h[i].ID.Bytes(), h[j].ID.Bytes()) == -1
}

// LeadingZeros returns the number of leading zeros in the digest of the given data.
func (h messageHeap) leadingZeros(data []byte) int {
	digest := blake2b.Sum512(data)
	asAnInt := new(big.Int).SetBytes(digest[:])
	return 8*blake2b.Size - asAnInt.BitLen()
}

// Swap swaps the elements with indexes i and j.
func (h messageHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

// Push adds x as the last element to the heap.
func (h *messageHeap) Push(x interface{}) {
	data := x.(*QueuedMessage)
	*h = append(*h, data)
	data.index = len(*h) - 1
}

// Pop removes and returns the last element of the heap.
func (h *messageHeap) Pop() interface{} {
	n := len(*h)
	data := (*h)[n-1]
	(*h)[n-1] = nil // avoid memory leak
	*h = (*h)[:n-1]
	data.index = -1
	return data
}

// interface contract (allow the compiler to check if the implementation has all the required methods).
var _ heap.Interface = &messageHeap{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ShutdownFlags ////////////////////////////////////////////////////////////////////////////////////////////////

// ShutdownFlag defines the type of the optional shutdown flags.
type ShutdownFlag = bitmask.BitMask

const (
	// CancelPendingElements defines a shutdown flag, that causes the queue to be emptied on shutdown.
	CancelPendingElements ShutdownFlag = 1 << iota

	// IgnorePendingTimeouts defines a shutdown flag, that makes the queue ignore the timeouts of the remaining queued
	// elements. Consecutive calls to Poll will immediately return these elements.
	IgnorePendingTimeouts

	// PanicOnModificationsAfterShutdown makes the queue panic instead of ignoring consecutive writes or modifications.
	PanicOnModificationsAfterShutdown
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// Option //////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Option is the type for functional options of the TimedQueue.
type Option func(queue *MessageQueue)

// WithMaxSize is an Option for the TimedQueue that allows to specify a maxSize of the queue.
func WithMaxSize(maxSize int) Option {
	return func(queue *MessageQueue) {
		queue.maxSize = maxSize
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////