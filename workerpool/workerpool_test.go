package workerpool

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Функция для передачи в воркерпул для тестирования
func testProcessFunc(workerID uint32, data string) string {
	return fmt.Sprintf("Worker %d: %s", workerID, data)
}

// Функция с задержкой для тестирования конкурентности
func slowProcessFunc(workerID uint32, data string) string {
	time.Sleep(10 * time.Millisecond)
	return fmt.Sprintf("Worker %d: %s", workerID, data)
}

func TestNewWorkerPool(t *testing.T) {
	inputChan := make(chan string, 10)
	defer close(inputChan)

	pool := NewWorkerPool(inputChan, 3, testProcessFunc)
	defer pool.ClosePool()

	if pool == nil {
		t.Fatal("NewWorkerPool returned nil")
	}

	output := pool.Output()
	if output == nil {
		t.Fatal("Output channel is nil")
	}
}

func TestAddWorker(t *testing.T) {
	inputChan := make(chan string, 10)
	defer close(inputChan)

	pool := NewWorkerPool(inputChan, 2, testProcessFunc)
	defer pool.ClosePool()

	// Добавляем нового воркера с уникальным ID
	err := pool.AddWorker(100)
	if err != nil {
		t.Fatalf("Failed to add worker: %v", err)
	}

	// Пытаемся добавить воркера с тем же ID
	err = pool.AddWorker(100)
	if err != ErrWorkerExists {
		t.Fatalf("Expected ErrWorkerExists, got: %v", err)
	}
}

func TestStopWorker(t *testing.T) {
	inputChan := make(chan string, 10)
	defer close(inputChan)

	pool := NewWorkerPool(inputChan, 3, testProcessFunc)
	defer pool.ClosePool()

	// Останавливаем существующего воркера
	err := pool.StopWorker(0)
	if err != nil {
		t.Fatalf("Failed to stop worker: %v", err)
	}

	// Пытаемся остановить несуществующего воркера
	err = pool.StopWorker(999)
	if err != ErrWorkerNotExists {
		t.Fatalf("Expected ErrWorkerNotExists, got: %v", err)
	}

	// Пытаемся остановить уже остановленного воркера
	err = pool.StopWorker(0)
	if err != ErrWorkerNotExists {
		t.Fatalf("Expected ErrWorkerNotExists for already stopped worker, got: %v", err)
	}
}

func TestConcurrentProcessing(t *testing.T) {
	inputChan := make(chan string, 100)
	pool := NewWorkerPool(inputChan, 3, slowProcessFunc)
	defer pool.ClosePool()

	numTasks := 20

	go func() {
		defer close(inputChan)
		for i := 0; i < numTasks; i++ {
			inputChan <- fmt.Sprintf("task-%d", i)
		}
	}()

	var results []string
	timeout := time.After(10 * time.Second)

	for len(results) < numTasks {
		select {
		case result, ok := <-pool.Output():
			if !ok {
				break
			}
			results = append(results, result)
		case <-timeout:
			t.Fatalf("Test timed out, got %d results out of %d", len(results), numTasks)
		}
	}

	if len(results) != numTasks {
		t.Fatalf("Expected %d results, got %d", numTasks, len(results))
	}
}

func TestDynamicWorkerManagement(t *testing.T) {
	inputChan := make(chan string, 100)
	pool := NewWorkerPool(inputChan, 2, testProcessFunc)
	defer pool.ClosePool()

	// Добавляем воркеров с уникальными ID
	for i := uint32(10); i < 15; i++ {
		err := pool.AddWorker(i)
		if err != nil {
			t.Fatalf("Failed to add worker %d: %v", i, err)
		}
	}

	// Останавливаем некоторых воркеров
	err := pool.StopWorker(0)
	if err != nil {
		t.Fatalf("Failed to stop worker 0: %v", err)
	}

	err = pool.StopWorker(12)
	if err != nil {
		t.Fatalf("Failed to stop worker 12: %v", err)
	}

	numTasks := 10
	go func() {
		defer close(inputChan)
		for i := 0; i < numTasks; i++ {
			inputChan <- fmt.Sprintf("test-%d", i)
		}
	}()

	// Собираем результаты с таймаутом
	var results []string
	timeout := time.After(5 * time.Second)

	for len(results) < numTasks {
		select {
		case result, ok := <-pool.Output():
			if !ok {
				break
			}
			results = append(results, result)
		case <-timeout:
			t.Fatalf("Test timed out, got %d results out of %d", len(results), numTasks)
		}
	}

	if len(results) != numTasks {
		t.Fatalf("Expected %d results, got %d", len(results), numTasks)
	}
}

func TestClosePool(t *testing.T) {
	inputChan := make(chan string, 10)
	pool := NewWorkerPool(inputChan, 2, testProcessFunc)

	pool.ClosePool()

	// Пытаемся добавить воркера после закрытия
	err := pool.AddWorker(100)
	if err != ErrClosed {
		t.Fatalf("Expected ErrClosed, got: %v", err)
	}

	// Проверяем, что output канал закрыт
	select {
	case _, ok := <-pool.Output():
		if ok {
			t.Fatal("Output channel should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Output channel should be closed immediately")
	}

	// Повторное закрытие
	pool.ClosePool()

	close(inputChan)
}

func TestRaceConditions(t *testing.T) {
	inputChan := make(chan string, 100)
	pool := NewWorkerPool(inputChan, 5, testProcessFunc)
	defer pool.ClosePool()

	var wg sync.WaitGroup
	numGoroutines := 5

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				workerID := uint32(id*10 + j + 100)
				pool.AddWorker(workerID)
			}
		}(i)
	}

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			for j := 0; j < 2; j++ {
				workerID := uint32(id*10 + j + 100)
				pool.StopWorker(workerID)
			}
		}(i)
	}

	go func() {
		defer close(inputChan)
		for i := 0; i < 20; i++ {
			inputChan <- fmt.Sprintf("race-test-%d", i)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()

	var results []string
	timeout := time.After(5 * time.Second)

	for {
		select {
		case result, ok := <-pool.Output():
			if !ok {
				goto checkResults
			}
			results = append(results, result)
		case <-timeout:
			goto checkResults
		}
	}

checkResults:
	// Проверяем полученные результаты
	if len(results) == 0 {
		t.Fatal("Expected some results from race condition test")
	}
}
