# A benchmark and playground for Completely Fair Scheduling in Go

[![Go Reference](https://pkg.go.dev/badge/github.com/tommie/go-coopsched.svg)](https://pkg.go.dev/github.com/tommie/go-coopsched)

See https://github.com/golang/go/issues/51071. I just wanted to play
around with it myself.

## Notes about hnes/cpuworker

The issue uses https://github.com/hnes/cpuworker as the testbed. Here
are some notes about the gist of it. This repository attempts to use
the same test-cases, but simplify the design to allow easier
experimentation. It also has self-contained Go benchmarks.

* Event-intensive tasks emulate goroutines with a lot of I/O wait.
* The `eventRoutineCall` subsystem lets the caller emulate I/O wait
  time.
* The user decides if they want the new task to be an
  "event-intensive" or "CPU" task, using `eiFlag` to `Submit3`. But
  this only affects the initial scheduling. Any yielding thereafter
  uses the computed event-intensity factor (EI-factor).
* The output of `calcEIfactorAndSumbitToRunnableTaskQueue` is directly
  used as priority for event-intensive tasks. The scheduler uses the
  following task priorities: event-intensive (PQ by EI-factor),
  new-task, CPU-intensive (based on EI-factor being zero, FIFO).
* Higher EI-factor means higher priority.
* Checkpoint is yielding, and the task's priority is decided by the
  EI-factor.
* Event-intensivity also determines the maximum time slice given.

## Differences to hnes/cpuworker

* All tasks ride on the same time slot expiration schedule.
* Event time is renamed "wait time".
* New tasks have highest priority, instead of sitting between
  wait-intensive and CPU-intensive.
* The EI-factor is based on `w / (w + r)` instead of `w / r`, to have
  a bounded value.
* The factor calculation never resets the accumulated time buckets,
  thus it's less likely to change priority quickly.
* There is no explicit `P`, instead replaced by a `numRunning` counter.

## Benchmark Design

These benchmarks use `b.N` to run a number of complete tests where 100
tasks per type are created and waited for. Each task performs some 100
operations, yielding in-between. The `noYield` versions run the same
code, but skips calling `Yield` and `Wait` (depending on task type.)

As an example, for the sleep task, a total of `100*100*b.N` calls to
`time.Sleep` are performed per benchmark run. This is similar to the
demo in hnes/cpuworker, except `ab` would continuously use 100
concurrent requests out of 10M instead of running separate batches.

## Initial Results (on a ThinkPad T460s laptop with external power)

The "avg delay overhead" is how long we waited in `<-time.After`,
after discounting the expected delay. Running time and blocking time
are total times for all scheduled goroutines. Note that no-yield still
blocks at the start of a goroutine, and that can take a long time if
the concurrency limit has been reached. "Avg load" is the average size
of the queue seen when taking the top task.

See below for full output. The case we care about is "mixed". No-yield
should all be equivalent, and never blocks in `Yield`. The numbers are
in the same ballpark, and the accuracy is fair:

```
BenchmarkFIFO/noYield/mixed-4     	       6	1982894160 ns/op	        27.61 avg-load	   1834925 block-µs/op	    193734 run-µs/op	     12337 wait-overhead-µs/op	         0 wait-µs/op
BenchmarkWaitiness/noYield/mixed-4         6	2000242534 ns/op	        27.60 avg-load	   1886621 block-µs/op	    195722 run-µs/op	     13489 wait-overhead-µs/op	         0 wait-µs/op
```

FIFO is what Go does, so we wouldn't expect that to be very different
from no-yield, except for more overhead. In the yield tests, the
channel sleep time is re-accounted as waiting time, instead of running
time. The much worse overhead may be due to the large load making the
scheduler a bottleneck, as indicated by the greater blocking time:

```
BenchmarkFIFO/yield/mixed-4       	       8	1289674838 ns/op	        17.97 avg-load	   2126736 block-µs/op	    102034 run-µs/op	   1140682 wait-overhead-µs/op	    129450 wait-µs/op
BenchmarkFIFO/noYield/mixed-4     	       6	1982894160 ns/op	        27.61 avg-load	   1834925 block-µs/op	    193734 run-µs/op	     12337 wait-overhead-µs/op	         0 wait-µs/op
```

Finally, `Waitiess` should give a lower overhead than FIFO, and that
should be due to lower blocking time. Note that delay overhead only
looks at the channel-intensive goroutines, not the CPU-intensive ones,
while both types yield to the scheduler. It does improve a factor of
5x for wait-overhead, and around 2x for blocking time:

```
BenchmarkFIFO/yield/mixed-4       	       8	1289674838 ns/op	        17.97 avg-load	   2126736 block-µs/op	    102034 run-µs/op	   1140682 wait-overhead-µs/op	    129450 wait-µs/op
BenchmarkWaitiness/yield/mixed-4  	       8	1310149936 ns/op	        38.20 avg-load	   1379195 block-µs/op	    102958 run-µs/op	    204797 wait-overhead-µs/op	    124973 wait-µs/op
```

## Conclusions

It took a couple of iterations (see Git history) to get this to do
what https://github.com/hnes/cpuworker does in terms of test
cases.

The load is high enough for a scheduling algorithm to have possible
impact.

## Go Scheduler Notes

* schedule (`runtime/proc.go`)
  * findRunnableGCWorker
  * globrunqget
    * A non-local `gQueue`, which is unbounded.
    * This call returns a `g`, but also fills `runq`.
  * runqget
    * A local circular buffer of 255/256 entries.
  * findrunnable
    * runqget
    * globrunqget
    * netpoll
      * Internal `g`s for network polling.
    * stealWork
      * Takes work from another thread/CPU.
      * stealOrder.start
      * The only order is `randomOrder`, which picks a `P` at
        pseudo-random.
      * checkTimers
        * runtimer
      * runqget
      * runqsteal
        * Takes half of the other `p`s runq.
   * `globrunqget` checked again.
* Go preemption timeslice är 10 ms.
* `schedEnabled` seems only used for GC pausing.
* `casgstatus` sets `runnableTime` if tracking is enabled.

## Output from Initial Benchmarking

Sixth version.

```console
$ go test -bench . -benchtime 10s ./
goos: linux
goarch: amd64
pkg: github.com/tommie/go-coopsched
cpu: Intel(R) Core(TM) i7-6600U CPU @ 2.60GHz
BenchmarkFIFO/yield/run-4         	      13	 912782670 ns/op	        25.52 avg-load	    780148 block-µs/op	     90906 run-µs/op	         0 wait-overhead-µs/op	         0 wait-µs/op
BenchmarkFIFO/yield/wait-4        	      67	 170363049 ns/op	         6.02 avg-load	     36457 block-µs/op	        94 run-µs/op	     62958 wait-overhead-µs/op	    127231 wait-µs/op
BenchmarkFIFO/yield/mixed-4       	       8	1289674838 ns/op	        17.97 avg-load	   2126736 block-µs/op	    102034 run-µs/op	   1140682 wait-overhead-µs/op	    129450 wait-µs/op
BenchmarkFIFO/noYield/run-4       	      12	 956969815 ns/op	        12.72 avg-load	    420550 block-µs/op	     92803 run-µs/op	         0 wait-overhead-µs/op	         0 wait-µs/op
BenchmarkFIFO/noYield/wait-4      	       8	1254513704 ns/op	        13.04 avg-load	    564536 block-µs/op	    125391 run-µs/op	     25351 wait-overhead-µs/op	         0 wait-µs/op
BenchmarkFIFO/noYield/mixed-4     	       6	1982894160 ns/op	        27.61 avg-load	   1834925 block-µs/op	    193734 run-µs/op	     12337 wait-overhead-µs/op	         0 wait-µs/op
BenchmarkWaitiness/yield/run-4    	      12	 915748782 ns/op	        25.59 avg-load	    783788 block-µs/op	     91213 run-µs/op	         0 wait-overhead-µs/op	         0 wait-µs/op
BenchmarkWaitiness/yield/wait-4   	      48	 214082004 ns/op	         5.31 avg-load	     32040 block-µs/op	        93 run-µs/op	     58974 wait-overhead-µs/op	    127550 wait-µs/op
BenchmarkWaitiness/yield/mixed-4  	       8	1310149936 ns/op	        38.20 avg-load	   1379195 block-µs/op	    102958 run-µs/op	    204797 wait-overhead-µs/op	    124973 wait-µs/op
BenchmarkWaitiness/noYield/run-4  	      12	 937030950 ns/op	        12.71 avg-load	    414003 block-µs/op	     91611 run-µs/op	         0 wait-overhead-µs/op	         0 wait-µs/op
BenchmarkWaitiness/noYield/wait-4 	       8	1264407792 ns/op	        13.05 avg-load	    569199 block-µs/op	    126408 run-µs/op	     26367 wait-overhead-µs/op	         0 wait-µs/op
BenchmarkWaitiness/noYield/mixed-4         6	2000242534 ns/op	        27.60 avg-load	   1886621 block-µs/op	    195722 run-µs/op	     13489 wait-overhead-µs/op	         0 wait-µs/op
PASS
ok  	github.com/tommie/go-coopsched	154.875s
```
