# A benchmark and playground for Completely Fair Scheduling in Go

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
* The EI-factor is based on `w / (w + r)` instead of `w / r`, to have
  a bounded value.
* The factor calculation never resets the accumulated time buckets,
  thus it's less likely to change priority quickly.
* There is no explicit `P`, instead replaced by a `numRunning` counter.

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
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:190: Avg delay overhead: 8.641522ms, avg running time: 231.800104ms, avg waiting time: 0s, avg blocking time: 53.515µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 9.32816ms, avg running time: 213.312939ms, avg waiting time: 0s, avg blocking time: 7.108740776s, avg load: 98.0
    coopsched_test.go:190: Avg delay overhead: 9.16891ms, avg running time: 214.27536ms, avg waiting time: 0s, avg blocking time: 23.544226144s, avg load: 331.5
BenchmarkFIFO/noYield/mixed-4     	     334	  71510421 ns/op

BenchmarkWaitness/noYield/mixed
    coopsched_test.go:190: Avg delay overhead: 8.099852ms, avg running time: 235.244041ms, avg waiting time: 0s, avg blocking time: 193.591µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 8.807681ms, avg running time: 213.335696ms, avg waiting time: 0s, avg blocking time: 7.011995933s, avg load: 97.5
    coopsched_test.go:190: Avg delay overhead: 8.946962ms, avg running time: 212.287389ms, avg waiting time: 0s, avg blocking time: 23.458988595s, avg load: 331.5
BenchmarkWaitness/noYield/mixed-4           	     334	  70840761 ns/op
```

FIFO is what Go does, so we wouldn't expect that to be very different
from no-yield, except for more overhead. In the yield tests, the
channel sleep time is re-accounted as waiting time, instead of running
time. The much worse overhead may be due to the large load making the
scheduler a bottleneck, as indicated by the greater blocking time:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:190: Avg delay overhead: 10.246976ms, avg running time: 136.791718ms, avg waiting time: 109.716914ms, avg blocking time: 613.792µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 4.618824677s, avg running time: 118.77109ms, avg waiting time: 125.86504ms, avg blocking time: 8.826939651s, avg load: 104.2
    coopsched_test.go:190: Avg delay overhead: 23.44885831s, avg running time: 119.524427ms, avg waiting time: 124.244686ms, avg blocking time: 44.700739169s, avg load: 584.8
BenchmarkFIFO/yield/mixed-4       	     490	  49732581 ns/op

BenchmarkFIFO/noYield/mixed
    coopsched_test.go:190: Avg delay overhead: 8.641522ms, avg running time: 231.800104ms, avg waiting time: 0s, avg blocking time: 53.515µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 9.32816ms, avg running time: 213.312939ms, avg waiting time: 0s, avg blocking time: 7.108740776s, avg load: 98.0
    coopsched_test.go:190: Avg delay overhead: 9.16891ms, avg running time: 214.27536ms, avg waiting time: 0s, avg blocking time: 23.544226144s, avg load: 331.5
BenchmarkFIFO/noYield/mixed-4     	     334	  71510421 ns/op
```

Finally, `Waitiess` should give a lower overhead than FIFO, and that
should be due to lower blocking time. Note that delay overhead only
looks at the channel-intensive goroutines, not the CPU-intensive ones,
while both types yield to the scheduler. It does improve a factor of
10x for overhead, and around 2x for blocking time:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:190: Avg delay overhead: 10.246976ms, avg running time: 136.791718ms, avg waiting time: 109.716914ms, avg blocking time: 613.792µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 4.618824677s, avg running time: 118.77109ms, avg waiting time: 125.86504ms, avg blocking time: 8.826939651s, avg load: 104.2
    coopsched_test.go:190: Avg delay overhead: 23.44885831s, avg running time: 119.524427ms, avg waiting time: 124.244686ms, avg blocking time: 44.700739169s, avg load: 584.8
BenchmarkFIFO/yield/mixed-4       	     490	  49732581 ns/op

BenchmarkWaitness/yield/mixed
    coopsched_test.go:190: Avg delay overhead: 10.652347ms, avg running time: 150.509321ms, avg waiting time: 110.139238ms, avg blocking time: 561.179µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 473.395621ms, avg running time: 124.917855ms, avg waiting time: 126.520127ms, avg blocking time: 5.213434736s, avg load: 136.9
    coopsched_test.go:190: Avg delay overhead: 2.321544413s, avg running time: 121.48177ms, avg waiting time: 127.387055ms, avg blocking time: 25.273282698s, avg load: 708.2
BenchmarkWaitness/yield/mixed-4   	     493	  46895588 ns/op
```

## Conclusions

It took a couple of iterations (see Git history) to get this to do
what https://github.com/hnes/cpuworker does in terms of test
cases. That code uses a `running / wait` priority, which is not yet
implemented here.

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
      * Internal `g`s for etwork polling.
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

Fifth version.

```console
$ go test -v -bench . -benchtime 20s ./
goos: linux
goarch: amd64
pkg: github.com/tommie/go-coopsched
cpu: Intel(R) Core(TM) i7-6600U CPU @ 2.60GHz
BenchmarkFIFO
BenchmarkFIFO/yield
BenchmarkFIFO/yield/run
    coopsched_test.go:133: Avg running time: 86.563174ms, avg waiting time: 0s, avg blocking time: 35.856µs, avg load: 1.0
    coopsched_test.go:133: Avg running time: 117.479254ms, avg waiting time: 0s, avg blocking time: 3.645990717s, avg load: 90.3
    coopsched_test.go:133: Avg running time: 116.053511ms, avg waiting time: 0s, avg blocking time: 22.397650729s, avg load: 557.7
BenchmarkFIFO/yield/run-4         	     609	  38749735 ns/op
BenchmarkFIFO/yield/wait
    coopsched_test.go:158: Avg delay overhead: 16.855053ms, avg running time: 140.267µs, avg waiting time: 116.120459ms, avg blocking time: 632.043µs, avg load: 1.0
    coopsched_test.go:158: Avg delay overhead: 457.640991ms, avg running time: 92.264µs, avg waiting time: 128.438049ms, avg blocking time: 430.204576ms, avg load: 74.7
    coopsched_test.go:158: Avg delay overhead: 28.473946484s, avg running time: 106.666µs, avg waiting time: 128.242901ms, avg blocking time: 28.586685482s, avg load: 4203.5
BenchmarkFIFO/yield/wait-4     	    4250	   6764641 ns/op
BenchmarkFIFO/yield/mixed
    coopsched_test.go:190: Avg delay overhead: 10.246976ms, avg running time: 136.791718ms, avg waiting time: 109.716914ms, avg blocking time: 613.792µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 4.618824677s, avg running time: 118.77109ms, avg waiting time: 125.86504ms, avg blocking time: 8.826939651s, avg load: 104.2
    coopsched_test.go:190: Avg delay overhead: 23.44885831s, avg running time: 119.524427ms, avg waiting time: 124.244686ms, avg blocking time: 44.700739169s, avg load: 584.8
BenchmarkFIFO/yield/mixed-4       	     490	  49732581 ns/op
BenchmarkFIFO/noYield
BenchmarkFIFO/noYield/run
    coopsched_test.go:133: Avg running time: 144.276303ms, avg waiting time: 0s, avg blocking time: 13.798µs, avg load: 1.0
    coopsched_test.go:133: Avg running time: 122.614947ms, avg waiting time: 0s, avg blocking time: 1.984100834s, avg load: 50.5
    coopsched_test.go:133: Avg running time: 121.49422ms, avg waiting time: 0s, avg blocking time: 11.77409351s, avg load: 289.5
BenchmarkFIFO/noYield/run-4       	     584	  40546861 ns/op
BenchmarkFIFO/noYield/wait
    coopsched_test.go:158: Avg delay overhead: 27.637555ms, avg running time: 127.66953ms, avg waiting time: 0s, avg blocking time: 3.931µs, avg load: 1.0
    coopsched_test.go:158: Avg delay overhead: 26.446431ms, avg running time: 126.485363ms, avg waiting time: 0s, avg blocking time: 2.03918686s, avg load: 50.5
    coopsched_test.go:158: Avg delay overhead: 26.568025ms, avg running time: 126.609601ms, avg waiting time: 0s, avg blocking time: 11.675367535s, avg load: 276.6
BenchmarkFIFO/noYield/wait-4   	     558	  42208863 ns/op
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:190: Avg delay overhead: 8.641522ms, avg running time: 231.800104ms, avg waiting time: 0s, avg blocking time: 53.515µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 9.32816ms, avg running time: 213.312939ms, avg waiting time: 0s, avg blocking time: 7.108740776s, avg load: 98.0
    coopsched_test.go:190: Avg delay overhead: 9.16891ms, avg running time: 214.27536ms, avg waiting time: 0s, avg blocking time: 23.544226144s, avg load: 331.5
BenchmarkFIFO/noYield/mixed-4     	     334	  71510421 ns/op
BenchmarkWaitness
BenchmarkWaitness/yield
BenchmarkWaitness/yield/run
    coopsched_test.go:133: Avg running time: 98.934403ms, avg waiting time: 0s, avg blocking time: 50.029µs, avg load: 1.0
    coopsched_test.go:133: Avg running time: 120.891455ms, avg waiting time: 0s, avg blocking time: 3.762729562s, avg load: 91.0
    coopsched_test.go:133: Avg running time: 120.159477ms, avg waiting time: 0s, avg blocking time: 22.786406628s, avg load: 548.5
BenchmarkWaitness/yield/run-4     	     594	  40126057 ns/op
BenchmarkWaitness/yield/wait
    coopsched_test.go:158: Avg delay overhead: 29.971548ms, avg running time: 91.723µs, avg waiting time: 129.357676ms, avg blocking time: 550.208µs, avg load: 1.0
    coopsched_test.go:158: Avg delay overhead: 249.458333ms, avg running time: 89.984µs, avg waiting time: 129.739856ms, avg blocking time: 225.757915ms, avg load: 40.6
    coopsched_test.go:158: Avg delay overhead: 13.183201578s, avg running time: 101.783µs, avg waiting time: 127.854098ms, avg blocking time: 13.288157823s, avg load: 2037.9
BenchmarkWaitness/yield/wait-4 	    4029	   6361365 ns/op
BenchmarkWaitness/yield/mixed
    coopsched_test.go:190: Avg delay overhead: 10.652347ms, avg running time: 150.509321ms, avg waiting time: 110.139238ms, avg blocking time: 561.179µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 473.395621ms, avg running time: 124.917855ms, avg waiting time: 126.520127ms, avg blocking time: 5.213434736s, avg load: 136.9
    coopsched_test.go:190: Avg delay overhead: 2.321544413s, avg running time: 121.48177ms, avg waiting time: 127.387055ms, avg blocking time: 25.273282698s, avg load: 708.2
BenchmarkWaitness/yield/mixed-4   	     493	  46895588 ns/op
BenchmarkWaitness/noYield
BenchmarkWaitness/noYield/run
    coopsched_test.go:133: Avg running time: 101.65178ms, avg waiting time: 0s, avg blocking time: 3.854µs, avg load: 1.0
    coopsched_test.go:133: Avg running time: 122.080144ms, avg waiting time: 0s, avg blocking time: 1.974096558s, avg load: 47.6
    coopsched_test.go:133: Avg running time: 122.605196ms, avg waiting time: 0s, avg blocking time: 11.849945822s, avg load: 289.5
BenchmarkWaitness/noYield/run-4   	     584	  40929987 ns/op
BenchmarkWaitness/noYield/wait
    coopsched_test.go:158: Avg delay overhead: 25.058955ms, avg running time: 125.086241ms, avg waiting time: 0s, avg blocking time: 6.004µs, avg load: 1.0
    coopsched_test.go:158: Avg delay overhead: 26.569652ms, avg running time: 126.608484ms, avg waiting time: 0s, avg blocking time: 2.046488741s, avg load: 47.6
    coopsched_test.go:158: Avg delay overhead: 26.376953ms, avg running time: 126.418227ms, avg waiting time: 0s, avg blocking time: 11.660306793s, avg load: 275.5
BenchmarkWaitness/noYield/wait-4         	     556	  42297174 ns/op
BenchmarkWaitness/noYield/mixed
    coopsched_test.go:190: Avg delay overhead: 8.099852ms, avg running time: 235.244041ms, avg waiting time: 0s, avg blocking time: 193.591µs, avg load: 1.0
    coopsched_test.go:190: Avg delay overhead: 8.807681ms, avg running time: 213.335696ms, avg waiting time: 0s, avg blocking time: 7.011995933s, avg load: 97.5
    coopsched_test.go:190: Avg delay overhead: 8.946962ms, avg running time: 212.287389ms, avg waiting time: 0s, avg blocking time: 23.458988595s, avg load: 331.5
BenchmarkWaitness/noYield/mixed-4           	     334	  70840761 ns/op
PASS
ok  	github.com/tommie/go-coopsched	343.093s
```
