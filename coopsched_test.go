package coopsched

import (
	"context"
	"hash/crc32"
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

	s := NewScheduler(0, RunningTimeFair)
	defer s.Close()

	var wg sync.WaitGroup

	wg.Add(1)
	s.Go(ctx, func(ctx context.Context) {
		defer wg.Done()

		for i := 0; i < 1000; i++ {
			Yield(ctx)
			// Do some piece of the computation.
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

func channelTask(ctx context.Context, amt int, yield bool) time.Duration {
	var wait time.Duration

	for i := 0; i < amt; i++ {
		start := time.Now()
		// Use time.After to make this a channel receive instead of
		// (possibly) using a native timer with time.Sleep.
		<-time.After(1 * time.Millisecond)
		wait += time.Now().Sub(start)

		if yield {
			Yield(ctx)
		}
	}

	return wait
}

func BenchmarkFIFO(b *testing.B) {
	b.Run("yield", func(b *testing.B) {
		doBenchmark(b, FIFO, true)
	})

	b.Run("noYield", func(b *testing.B) {
		doBenchmark(b, FIFO, false)
	})
}

func BenchmarkRunningTimeFair(b *testing.B) {
	b.Run("yield", func(b *testing.B) {
		doBenchmark(b, RunningTimeFair, true)
	})

	b.Run("noYield", func(b *testing.B) {
		doBenchmark(b, RunningTimeFair, false)
	})
}

func doBenchmark(b *testing.B, algo SchedulingAlgo, yield bool) {
	ctx := context.Background()

	const amt = 100

	b.Run("cpu", func(b *testing.B) {
		s := NewScheduler(0, algo)
		defer s.Close()

		var wg sync.WaitGroup

		wg.Add(b.N)
		for i := 0; i < b.N; i++ {
			s.Go(ctx, func(ctx context.Context) {
				defer wg.Done()

				cpuIntensiveTask(ctx, amt, yield)
			})
		}

		wg.Wait()

		b.Logf("Avg running time: %v, avg blocking time: %v, avg load: %.1f",
			s.RunningTime()/time.Duration(b.N),
			s.BlockingTime()/time.Duration(b.N),
			s.AvgLoad())
	})

	b.Run("channel", func(b *testing.B) {
		s := NewScheduler(0, algo)
		defer s.Close()

		var waitNS uint64
		var wg sync.WaitGroup

		wg.Add(b.N)
		for i := 0; i < b.N; i++ {
			s.Go(ctx, func(ctx context.Context) {
				defer wg.Done()

				atomic.AddUint64(&waitNS, uint64(channelTask(ctx, amt, yield)/time.Nanosecond))
			})
		}

		wg.Wait()

		b.Logf("Avg delay overhead: %v, avg running time: %v, avg blocking time: %v, avg load: %.1f",
			time.Duration(waitNS/uint64(b.N))*time.Nanosecond-amt*time.Millisecond,
			s.RunningTime()/time.Duration(b.N),
			s.BlockingTime()/time.Duration(b.N),
			s.AvgLoad())
	})

	b.Run("mixed", func(b *testing.B) {
		s := NewScheduler(0, algo)
		defer s.Close()

		var waitNS uint64
		var wg sync.WaitGroup

		wg.Add(2 * b.N)
		for i := 0; i < b.N; i++ {
			s.Go(ctx, func(ctx context.Context) {
				defer wg.Done()

				cpuIntensiveTask(ctx, amt, yield)
			})

			s.Go(ctx, func(ctx context.Context) {
				defer wg.Done()

				atomic.AddUint64(&waitNS, uint64(channelTask(ctx, amt, yield)/time.Nanosecond))
			})
		}

		wg.Wait()

		b.Logf("Avg delay overhead: %v, avg running time: %v, avg blocking time: %v, avg load: %.1f",
			time.Duration(waitNS/uint64(b.N))*time.Nanosecond-amt*time.Millisecond,
			s.RunningTime()/time.Duration(b.N),
			s.BlockingTime()/time.Duration(b.N),
			s.AvgLoad())
	})
}
