# WorkerPool на Go

Простая реализация пула воркеров с возможностью динамически добавлять и удалять воркеров.

## Быстрый пример

```go
input := make(chan string, 10)
pool := workerpool.NewWorkerPool(input, 4, processData)

go func() {
    for res := range pool.Output() {
        fmt.Println(res)
    }
}()

// отправляем задачи и управляем воркерами
input <- "task1"
pool.AddWorker(4)
pool.StopWorker(1)
close(input)           // больше задач нет
pool.ClosePool()       // ожидает окончания и закрывает Output
```

## API

```go
func NewWorkerPool(
    inputChan chan string,
    initialWorkers uint32,
    process func(workerID uint32, data string) string,
) WorkerpoolI
```

* `AddWorker(id uint32) error` — добавить воркера
* `StopWorker(id uint32) error` — остановить воркера
* `ClosePool()` — завершить пул (закрывает канал Output)
* `Output() <-chan string` — канал результатов

### Ошибки

* `ErrWorkerExists` — воркер с таким ID уже есть
* `ErrWorkerNotExists` — воркера нет
* `ErrClosed` — пул закрыт и нельзя менять воркеров

## Реализация

* Для каждого воркера при старте (`NewWorkerPool` и `AddWorker`) создаётся `context.WithCancel`, функция отмены сохраняется в `map[uint32]context.CancelFunc`.
* Мьютекс для безопасности: доступ к общей `map` и флагу `closed` защищён `sync.Mutex`.
* Воркер слушает `ctx.Done()` и завершается при вызове `StopWorker(id)` или при `ClosePool()`.
* В `ClosePool()` устанавливается флаг закрытия, ждутся все горутины (через `sync.WaitGroup`), а затем закрывается канал `Output`.

