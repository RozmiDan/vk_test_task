package workerpool

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrClosed          = errors.New("worker pool is closed")
	ErrWorkerExists    = errors.New("worker already exists")
	ErrWorkerNotExists = errors.New("worker does not exist")
)

type WorkerpoolI interface {
	AddWorker(workerID uint32) error
	StopWorker(workerID uint32) error
	ClosePool()
	Output() <-chan string
}

type WorkerPool struct {
	input  chan string
	output chan string
	// workerCount uint32
	functionF func(uint32, string) string
	mu        sync.Mutex
	cancels   map[uint32]context.CancelFunc
	wg        sync.WaitGroup
	closed    bool
}

func NewWorkerPool(inputChan chan string, counter uint32, function func(uint32, string) string) WorkerpoolI {
	wp := &WorkerPool{
		input:  inputChan,
		output: make(chan string, counter),
		// workerCount: counter,
		functionF: function,
		mu:        sync.Mutex{},
		cancels:   make(map[uint32]context.CancelFunc, counter),
		wg:        sync.WaitGroup{},
		closed:    false,
	}

	for i := uint32(0); i < counter; i++ {
		wp.AddWorker(i)
	}

	return wp
}

func (w *WorkerPool) AddWorker(workerID uint32) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrClosed
	}

	if _, ok := w.cancels[workerID]; ok {
		return ErrWorkerExists
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancels[workerID] = cancel

	w.wg.Add(1)
	go w.worker(ctx, workerID)
	// fmt.Printf("worker %d started", workerID)
	return nil
}

func (w *WorkerPool) StopWorker(workerID uint32) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	cancel, ok := w.cancels[workerID]
	if !ok {
		return ErrWorkerNotExists
	}

	delete(w.cancels, workerID)
	cancel()
	// fmt.Printf("Worker %d stopped\n", workerID)
	return nil
}

func (w *WorkerPool) worker(ctx context.Context, workerID uint32) {
	defer w.wg.Done()
	// defer fmt.Printf("Worker %d finished\n", workerID)

	for {
		select {
		case <-ctx.Done():
			return
		case val, ok := <-w.input:
			if !ok {
				return
			}

			result := w.functionF(workerID, val)

			select {
			case <-ctx.Done():
				return
			case w.output <- result:
			}
		}
	}
}

func (w *WorkerPool) ClosePool() {
	w.mu.Lock()

	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true

	for _, cancel := range w.cancels {
		cancel()
	}
	w.cancels = nil
	w.mu.Unlock()

	w.wg.Wait()
	close(w.output)
	//fmt.Println("Worker pool closed")
}

func (w *WorkerPool) Output() <-chan string {
	return w.output
}
