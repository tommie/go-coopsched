package coopsched

import (
	"context"
	"hash/crc32"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// cpuFactor is an `amt` coefficient to make cpuIntensiveTask be as
// slow as channelTask for the same amt.
const cpuFactor = 66

func ExampleScheduler() {
	ctx := context.TODO()

	s := NewScheduler(0, Waitiness)
	defer s.Close()

	var wg sync.WaitGroup

	wg.Add(1)
	go s.Do(ctx, func(ctx context.Context) {
		defer wg.Done()

		for i := 0; i < 1000; i++ {
			Yield(ctx)
			// Do some piece of the computation.
		}
	})

	wg.Add(1)
	go s.Do(ctx, func(ctx context.Context) {
		defer wg.Done()

		for i := 0; i < 1000; i++ {
			Wait(ctx, func() {
				// Do some I/O.
			})
		}
	})

	wg.Wait()
}

func cpuIntensiveTask(ctx context.Context, amt int, yield bool) {
	amt *= cpuFactor

	bs := make([]byte, 256*1024)
	var ck uint32
	for i := 0; i < amt; i++ {
		ck |= crc32.ChecksumIEEE(bs)
		if yield {
			Yield(ctx)
		}
	}
}

func sleepTask(ctx context.Context, amt int, wait bool) time.Duration {
	var waitDur time.Duration

	for i := 0; i < amt; i++ {
		start := time.Now()

		if wait {
			Wait(ctx, func() {
				time.Sleep(1 * time.Millisecond)
			})
		} else {
			time.Sleep(1 * time.Millisecond)
		}

		waitDur += time.Now().Sub(start)
	}

	return waitDur - time.Duration(amt)*time.Millisecond
}

func BenchmarkFIFO(b *testing.B) {
	b.Run("yield", func(b *testing.B) {
		doBenchmark(b, FIFO, true)
	})

	b.Run("noYield", func(b *testing.B) {
		doBenchmark(b, FIFO, false)
	})
}

func BenchmarkWaitness(b *testing.B) {
	b.Run("yield", func(b *testing.B) {
		doBenchmark(b, Waitiness, true)
	})

	b.Run("noYield", func(b *testing.B) {
		doBenchmark(b, Waitiness, false)
	})
}

func doBenchmark(b *testing.B, algo SchedulingAlgo, yield bool) {
	ctx := context.Background()

	const amt = 100

	conc := runtime.GOMAXPROCS(0) - 1
	if !yield {
		// When using the scheduler, we reserve one core for
		// bookkeeping, so we need to do the same when running the
		// tests without yielding to the scheduler.
		oldConc := runtime.GOMAXPROCS(conc)
		b.Cleanup(func() { runtime.GOMAXPROCS(oldConc) })
	}

	b.Run("run", func(b *testing.B) {
		s := NewScheduler(conc, algo)
		defer s.Close()

		var wg sync.WaitGroup

		wg.Add(b.N)
		for i := 0; i < b.N; i++ {
			go s.Do(ctx, func(ctx context.Context) {
				defer wg.Done()

				cpuIntensiveTask(ctx, amt, yield)
			})
		}

		wg.Wait()

		b.Logf("Avg running time: %v, avg waiting time: %v, avg blocking time: %v, avg load: %.1f",
			s.RunningTime()/time.Duration(b.N),
			s.WaitingTime()/time.Duration(b.N),
			s.BlockingTime()/time.Duration(b.N),
			s.AvgLoad())
	})

	b.Run("wait", func(b *testing.B) {
		s := NewScheduler(conc, algo)
		defer s.Close()

		var waitNS uint64
		var wg sync.WaitGroup

		wg.Add(b.N)
		for i := 0; i < b.N; i++ {
			go s.Do(ctx, func(ctx context.Context) {
				defer wg.Done()

				atomic.AddUint64(&waitNS, uint64(sleepTask(ctx, amt, yield)/time.Nanosecond))
			})
		}

		wg.Wait()

		b.Logf("Avg delay overhead: %v, avg running time: %v, avg waiting time: %v, avg blocking time: %v, avg load: %.1f",
			time.Duration(waitNS/uint64(b.N))*time.Nanosecond,
			s.RunningTime()/time.Duration(b.N),
			s.WaitingTime()/time.Duration(b.N),
			s.BlockingTime()/time.Duration(b.N),
			s.AvgLoad())
	})

	b.Run("mixed", func(b *testing.B) {
		s := NewScheduler(conc, algo)
		defer s.Close()

		var waitNS uint64
		var wg sync.WaitGroup

		wg.Add(2 * b.N)
		for i := 0; i < b.N; i++ {
			go s.Do(ctx, func(ctx context.Context) {
				defer wg.Done()

				cpuIntensiveTask(ctx, amt, yield)
			})

			go s.Do(ctx, func(ctx context.Context) {
				defer wg.Done()

				atomic.AddUint64(&waitNS, uint64(sleepTask(ctx, amt, yield)/time.Nanosecond))
			})
		}

		wg.Wait()

		b.Logf("Avg delay overhead: %v, avg running time: %v, avg waiting time: %v, avg blocking time: %v, avg load: %.1f",
			time.Duration(waitNS/uint64(b.N))*time.Nanosecond,
			s.RunningTime()/time.Duration(b.N),
			s.WaitingTime()/time.Duration(b.N),
			s.BlockingTime()/time.Duration(b.N),
			s.AvgLoad())
	})
}
