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

const (
	// cpuFactor is an `amt` coefficient to make cpuIntensiveTask be as
	// slow as channelTask for the same amt.
	cpuFactor = 50

	verboseBenchmarks = false
)

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

func BenchmarkFIFO(b *testing.B) {
	b.Run("yield", func(b *testing.B) {
		doBenchmark(b, FIFO, true)
	})

	b.Run("noYield", func(b *testing.B) {
		doBenchmark(b, FIFO, false)
	})
}

func BenchmarkWaitiness(b *testing.B) {
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

	numTasks := 10 * conc

	b.Run("run", func(b *testing.B) {
		s := NewScheduler(conc, algo)
		defer s.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup

			wg.Add(numTasks)
			for i := 0; i < numTasks; i++ {
				go s.Do(ctx, func(ctx context.Context) {
					defer wg.Done()

					cpuIntensiveTask(ctx, amt, yield)
				})
			}

			wg.Wait()
		}

		reportScheduler(b, s, numTasks, 0)
	})

	b.Run("wait", func(b *testing.B) {
		s := NewScheduler(conc, algo)
		defer s.Close()

		b.ResetTimer()
		var waitOverheadNS uint64
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup

			wg.Add(numTasks)
			for i := 0; i < numTasks; i++ {
				go s.Do(ctx, func(ctx context.Context) {
					defer wg.Done()

					atomic.AddUint64(&waitOverheadNS, uint64(sleepTask(ctx, amt, yield)/time.Nanosecond))
				})
			}

			wg.Wait()
		}

		reportScheduler(b, s, numTasks, waitOverheadNS)
	})

	b.Run("mixed", func(b *testing.B) {
		s := NewScheduler(conc, algo)
		defer s.Close()

		b.ResetTimer()
		var waitOverheadNS uint64
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup

			wg.Add(2 * numTasks)
			for i := 0; i < numTasks; i++ {
				go s.Do(ctx, func(ctx context.Context) {
					defer wg.Done()

					cpuIntensiveTask(ctx, amt, yield)
				})

				go s.Do(ctx, func(ctx context.Context) {
					defer wg.Done()

					atomic.AddUint64(&waitOverheadNS, uint64(sleepTask(ctx, amt, yield)/time.Nanosecond))
				})
			}

			wg.Wait()
		}

		reportScheduler(b, s, numTasks, waitOverheadNS)
	})
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

func reportScheduler(b *testing.B, s *Scheduler, numTasks int, waitOverheadNS uint64) {
	b.StopTimer()
	defer b.StartTimer()

	rt := s.RunningTime() / time.Duration(b.N*numTasks)
	wt := s.WaitingTime() / time.Duration(b.N*numTasks)
	bt := s.BlockingTime() / time.Duration(b.N*numTasks)
	waitOverhead := time.Duration(waitOverheadNS) * time.Nanosecond / time.Duration(uint64(b.N*numTasks))

	b.ReportMetric(float64(rt/time.Microsecond), "run-µs/op")
	b.ReportMetric(float64(wt/time.Microsecond), "wait-µs/op")
	b.ReportMetric(float64(bt/time.Microsecond), "block-µs/op")
	b.ReportMetric(float64(waitOverhead/time.Microsecond), "wait-overhead-µs/op")
	b.ReportMetric(float64(s.AvgLoad()), "avg-load")

	if verboseBenchmarks {
		b.Logf("Avg delay overhead: %v, avg running time: %v, avg waiting time: %v, avg blocking time: %v, avg load: %.1f",
			waitOverhead, rt, wt, bt, s.AvgLoad())
	}
}
