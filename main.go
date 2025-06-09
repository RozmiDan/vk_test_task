package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/RozmiDan/vk_test_task/workerpool"
)

func main() {
	wg := &sync.WaitGroup{}
	inputChan := make(chan string, 10)

	pool := workerpool.NewWorkerPool(inputChan, 4, processData)

	wg.Add(1)

	go func() {
		for result := range pool.Output() {
			fmt.Println("Result:", result)
		}
		fmt.Println("Output channel closed")
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			inputChan <- fmt.Sprintf("task-%d", i)
			time.Sleep(50 * time.Millisecond)
		}

		time.Sleep(200 * time.Millisecond)
		fmt.Println("\n--- Adding new worker ---")
		if err := pool.AddWorker(4); err != nil {
			fmt.Printf("Error adding worker: %v\n", err)
		}

		for i := 10; i < 30; i++ {
			inputChan <- fmt.Sprintf("task-%d", i)
			time.Sleep(50 * time.Millisecond)
		}

		time.Sleep(200 * time.Millisecond)
		fmt.Println("\n--- Removing worker ---")
		if err := pool.StopWorker(2); err != nil {
			fmt.Printf("Error stopping worker: %v\n", err)
		}

		for i := 30; i < 50; i++ {
			inputChan <- fmt.Sprintf("task-%d", i)
			time.Sleep(50 * time.Millisecond)
		}

		close(inputChan)
	}()

	wg.Wait()

	pool.ClosePool()

	time.Sleep(500 * time.Millisecond)
}

func processData(workerID uint32, data string) string {
	time.Sleep(100 * time.Millisecond)
	return fmt.Sprintf("Worker %d processed: %s", workerID, data)
}
