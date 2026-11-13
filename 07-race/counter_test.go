package counter

import "testing"

const (
	goroutines = 1000
	perG       = 1000
	want       = goroutines * perG // 1_000_000
)

// TestCorrectVersions: все три исправленные версии дают точный результат.
// Запускай с -race: детектор должен молчать на них.
//
//	go test -race ./07-race
func TestCorrectVersions(t *testing.T) {
	versions := map[string]func(int, int) int{
		"mutex":  MutexCount,
		"atomic": AtomicCount,
		"chan":   ChanCount,
	}
	for name, fn := range versions {
		t.Run(name, func(t *testing.T) {
			if got := fn(goroutines, perG); got != want {
				t.Errorf("%s = %d, want %d", name, got, want)
			}
		})
	}
}

// TestRacyIsWrong фиксирует, что сломанная версия теряет инкременты.
// Под -race детектор валит прогон RacyCount по дизайну, поэтому тест
// пропускается (isRaceEnabled приходит из race_{enabled,disabled}_test.go
// по build-тегу //go:build race).
func TestRacyIsWrong(t *testing.T) {
	if isRaceEnabled {
		t.Skip("RacyCount намеренно гоняет данные; под -race детектор валит тест")
	}
	got := RacyCount(goroutines, perG)
	if got == want {
		t.Logf("повезло: racy дал точный %d (бывает редко), обычно меньше", got)
	} else {
		t.Logf("racy потерял инкременты: %d вместо %d — это и есть гонка", got, want)
	}
}

// Бенчмарки сравнивают скорость трёх версий. Запуск:
//
//	go test -bench=. -benchmem ./07-race
//
// b.N подбирает сам runtime; внутри — та же нагрузка (1000x1000).
// Вывод (ns/op) перенеси в README с выводом, когда какой способ уместен.

func BenchmarkMutex(b *testing.B) {
	for b.Loop() {
		MutexCount(goroutines, perG)
	}
}

func BenchmarkAtomic(b *testing.B) {
	for b.Loop() {
		AtomicCount(goroutines, perG)
	}
}

func BenchmarkChan(b *testing.B) {
	for b.Loop() {
		ChanCount(goroutines, perG)
	}
}
