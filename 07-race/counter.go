// Package counter — четыре счётчика, которые 1000 горутин увеличивают по 1000 раз.
//
// Задание 7: показать data race и починить тремя способами (Mutex, atomic, канал).
// Все исправленные версии при goroutines*perG == 1_000_000 обязаны вернуть ровно это.
package counter

import (
	"sync"
	"sync/atomic"
)

// run запускает goroutines штук горутин, каждая вызывает inc() perG раз,
// и ждёт их завершения. Общий каркас для всех версий — меняется только inc.
func run(goroutines, perG int, inc func()) {
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				inc()
			}
		}()
	}
	wg.Wait()
}

// RacyCount — НАРОЧНО сломанная версия: общий int без синхронизации.
// Запусти тест с -race: детектор покажет гонку, а результат будет меньше
// goroutines*perG (часть ++ потеряется). Чинить её НЕ надо — она эталон «как нельзя».
//
// Почему теряется: count++ это три шага (прочитать, +1, записать). Две горутины
// читают одно значение, прибавляют, пишут — одно из двух увеличений пропадает.
func RacyCount(goroutines, perG int) int {
	count := 0
	run(goroutines, perG, func() {
		count++
	})
	return count
}

// MutexCount — версия с sync.Mutex: инкремент под замком.
// Mutex даёт эксклюзивный доступ: пока одна горутина держит Lock, остальные ждут.
func MutexCount(goroutines, perG int) int {
	var mu sync.Mutex
	count := 0
	run(goroutines, perG, func() {
		mu.Lock()
		count++
		mu.Unlock()
	})
	return count
}

// AtomicCount — версия с sync/atomic: инкремент одной атомарной операцией.
// atomic делает «прочитать-прибавить-записать» неделимым на уровне процессора,
// без блокировки. Подходит для простого счётчика; для сложной логики — Mutex.
func AtomicCount(goroutines, perG int) int {
	var count atomic.Int64
	run(goroutines, perG, func() {
		count.Add(1)
	})
	return int(count.Load())
}

// ChanCount — версия на канале: горутины шлют по единице, один сборщик суммирует.
// Идиома «один владелец данных»: total трогает только горутина-сборщик,
// остальные общаются через канал — гонки нет по построению.
// Самый медленный способ для счётчика (накладные расходы на передачу),
// но показывает подход «share memory by communicating».
func ChanCount(goroutines, perG int) int {
	ch := make(chan int)
	done := make(chan int)
	go func() { // единственный, кто пишет в total
		total := 0
		for n := range ch {
			total += n
		}
		done <- total
	}()
	run(goroutines, perG, func() { ch <- 1 })
	close(ch) // range в сборщике завершится
	return <-done
}
