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

## Initial Results (on a ThinkPad T460s laptop)

The "avg delay overhead" is how long we waited in `<-time.After`,
after discounting the expected delay. Running time and blocking time
are total times for all scheduled goroutines. Note that no-yield still
blocks at the start of a goroutine, and that can take a long time if
the concurrency limit has been reached. "Avg load" is the average size
of the queue seen when taking the top task.

See below for full output. The case we care about is "mixed". No-yield
should all be equivalent, and never blocks in `Yield`. The numbers are
in the same ballpark, but the accuracy isn't very high:

```
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:180: Avg delay overhead: 11.021367ms, avg running time: 235.869947ms, avg waiting time: 0s, avg blocking time: 40.855µs, avg load: 1.0
    coopsched_test.go:180: Avg delay overhead: 10.189435ms, avg running time: 215.918835ms, avg waiting time: 0s, avg blocking time: 7.130776699s, avg load: 97.5
    coopsched_test.go:180: Avg delay overhead: 8.637328ms, avg running time: 213.072672ms, avg waiting time: 0s, avg blocking time: 23.496458202s, avg load: 328.5
BenchmarkFIFO/noYield/mixed-4         	     331	  71263245 ns/op

BenchmarkRunningTimeFair/noYield/mixed
    coopsched_test.go:180: Avg delay overhead: 9.222699ms, avg running time: 238.357244ms, avg waiting time: 0s, avg blocking time: 33.846µs, avg load: 1.5
    coopsched_test.go:180: Avg delay overhead: 14.407771ms, avg running time: 227.817446ms, avg waiting time: 0s, avg blocking time: 7.446002598s, avg load: 97.5
    coopsched_test.go:180: Avg delay overhead: 13.883597ms, avg running time: 223.97811ms, avg waiting time: 0s, avg blocking time: 23.500702549s, avg load: 311.1
BenchmarkRunningTimeFair/noYield/mixed-4         	     313	  74731269 ns/op
```

FIFO is what Go does, so we wouldn't expect that to be very different
from no-yield, except for more overhead. The lack of priority for
newly created tasks shows clearly:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:180: Avg delay overhead: 9.006743ms, avg running time: 133.975184ms, avg waiting time: 108.625697ms, avg blocking time: 418.512µs, avg load: 1.0
    coopsched_test.go:180: Avg delay overhead: 4.638750197s, avg running time: 118.822035ms, avg waiting time: 126.441162ms, avg blocking time: 8.83880514s, avg load: 105.0
    coopsched_test.go:180: Avg delay overhead: 23.890734477s, avg running time: 121.117635ms, avg waiting time: 126.660792ms, avg blocking time: 45.555435711s, avg load: 584.6
BenchmarkFIFO/yield/mixed-4           	     488	  50824276 ns/op

BenchmarkFIFO/noYield/mixed
    coopsched_test.go:180: Avg delay overhead: 11.021367ms, avg running time: 235.869947ms, avg waiting time: 0s, avg blocking time: 40.855µs, avg load: 1.0
    coopsched_test.go:180: Avg delay overhead: 10.189435ms, avg running time: 215.918835ms, avg waiting time: 0s, avg blocking time: 7.130776699s, avg load: 97.5
    coopsched_test.go:180: Avg delay overhead: 8.637328ms, avg running time: 213.072672ms, avg waiting time: 0s, avg blocking time: 23.496458202s, avg load: 328.5
BenchmarkFIFO/noYield/mixed-4         	     331	  71263245 ns/op
```

Finally, RunningTimeFair should give a lower overhead than FIFO. For a
load > 1, it has a clearly positive impact:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:180: Avg delay overhead: 9.006743ms, avg running time: 133.975184ms, avg waiting time: 108.625697ms, avg blocking time: 418.512µs, avg load: 1.0
    coopsched_test.go:180: Avg delay overhead: 4.638750197s, avg running time: 118.822035ms, avg waiting time: 126.441162ms, avg blocking time: 8.83880514s, avg load: 105.0
    coopsched_test.go:180: Avg delay overhead: 23.890734477s, avg running time: 121.117635ms, avg waiting time: 126.660792ms, avg blocking time: 45.555435711s, avg load: 584.6
BenchmarkFIFO/yield/mixed-4           	     488	  50824276 ns/op

BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:180: Avg delay overhead: 24.038897ms, avg running time: 196.19617ms, avg waiting time: 123.267015ms, avg blocking time: 869.139µs, avg load: 1.0
    coopsched_test.go:180: Avg delay overhead: 672.438526ms, avg running time: 123.338141ms, avg waiting time: 128.568946ms, avg blocking time: 5.277720363s, avg load: 165.7
    coopsched_test.go:180: Avg delay overhead: 3.896321999s, avg running time: 122.752475ms, avg waiting time: 127.077907ms, avg blocking time: 27.963958195s, avg load: 916.2
BenchmarkRunningTimeFair/yield/mixed-4           	     504	  47866332 ns/op
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

Fourth version.

```console
$ go test -v -bench . -benchtime 20s ./
goos: linux
goarch: amd64
pkg: github.com/tommie/go-coopsched
cpu: Intel(R) Core(TM) i7-6600U CPU @ 2.60GHz
BenchmarkFIFO
BenchmarkFIFO/yield
BenchmarkFIFO/yield/cpu
    coopsched_test.go:123: Avg running time: 92.102604ms, avg waiting time: 0s, avg blocking time: 47.596µs, avg load: 1.0
    coopsched_test.go:123: Avg running time: 115.943773ms, avg waiting time: 0s, avg blocking time: 3.588551684s, avg load: 90.3
    coopsched_test.go:123: Avg running time: 116.205325ms, avg waiting time: 0s, avg blocking time: 22.910487414s, avg load: 569.6
BenchmarkFIFO/yield/cpu-4    	     619	  38803705 ns/op
BenchmarkFIFO/yield/channel
    coopsched_test.go:148: Avg delay overhead: 27.517134ms, avg running time: 92.885µs, avg waiting time: 127.025936ms, avg blocking time: 423.734µs, avg load: 1.0
    coopsched_test.go:148: Avg delay overhead: 484.062317ms, avg running time: 89.782µs, avg waiting time: 127.257488ms, avg blocking time: 459.139362ms, avg load: 75.4
    coopsched_test.go:148: Avg delay overhead: 27.355752281s, avg running time: 106.518µs, avg waiting time: 127.943391ms, avg blocking time: 27.439068972s, avg load: 4015.2
BenchmarkFIFO/yield/channel-4         	    4066	   6793184 ns/op
BenchmarkFIFO/yield/mixed
    coopsched_test.go:180: Avg delay overhead: 9.006743ms, avg running time: 133.975184ms, avg waiting time: 108.625697ms, avg blocking time: 418.512µs, avg load: 1.0
    coopsched_test.go:180: Avg delay overhead: 4.638750197s, avg running time: 118.822035ms, avg waiting time: 126.441162ms, avg blocking time: 8.83880514s, avg load: 105.0
    coopsched_test.go:180: Avg delay overhead: 23.890734477s, avg running time: 121.117635ms, avg waiting time: 126.660792ms, avg blocking time: 45.555435711s, avg load: 584.6
BenchmarkFIFO/yield/mixed-4           	     488	  50824276 ns/op
BenchmarkFIFO/noYield
BenchmarkFIFO/noYield/cpu
    coopsched_test.go:123: Avg running time: 130.616801ms, avg waiting time: 0s, avg blocking time: 8.101µs, avg load: 1.0
    coopsched_test.go:123: Avg running time: 121.135559ms, avg waiting time: 0s, avg blocking time: 1.963583325s, avg load: 47.6
    coopsched_test.go:123: Avg running time: 122.49682ms, avg waiting time: 0s, avg blocking time: 11.909576636s, avg load: 291.5
BenchmarkFIFO/noYield/cpu-4           	     588	  40898487 ns/op
BenchmarkFIFO/noYield/channel
    coopsched_test.go:148: Avg delay overhead: 28.122149ms, avg running time: 128.16993ms, avg waiting time: 0s, avg blocking time: 6.667µs, avg load: 1.0
    coopsched_test.go:148: Avg delay overhead: 25.712499ms, avg running time: 125.753658ms, avg waiting time: 0s, avg blocking time: 2.021088017s, avg load: 50.5
    coopsched_test.go:148: Avg delay overhead: 25.589067ms, avg running time: 125.632851ms, avg waiting time: 0s, avg blocking time: 11.641093597s, avg load: 277.5
BenchmarkFIFO/noYield/channel-4       	     560	  41955488 ns/op
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:180: Avg delay overhead: 11.021367ms, avg running time: 235.869947ms, avg waiting time: 0s, avg blocking time: 40.855µs, avg load: 1.0
    coopsched_test.go:180: Avg delay overhead: 10.189435ms, avg running time: 215.918835ms, avg waiting time: 0s, avg blocking time: 7.130776699s, avg load: 97.5
    coopsched_test.go:180: Avg delay overhead: 8.637328ms, avg running time: 213.072672ms, avg waiting time: 0s, avg blocking time: 23.496458202s, avg load: 328.5
BenchmarkFIFO/noYield/mixed-4         	     331	  71263245 ns/op
BenchmarkRunningTimeFair
BenchmarkRunningTimeFair/yield
BenchmarkRunningTimeFair/yield/cpu
    coopsched_test.go:123: Avg running time: 137.295167ms, avg waiting time: 0s, avg blocking time: 96.913µs, avg load: 1.0
    coopsched_test.go:123: Avg running time: 120.510084ms, avg waiting time: 0s, avg blocking time: 3.779803742s, avg load: 91.8
    coopsched_test.go:123: Avg running time: 121.324648ms, avg waiting time: 0s, avg blocking time: 23.103885433s, avg load: 550.3
BenchmarkRunningTimeFair/yield/cpu-4  	     595	  40549987 ns/op
BenchmarkRunningTimeFair/yield/channel
    coopsched_test.go:148: Avg delay overhead: 23.844045ms, avg running time: 172.855µs, avg waiting time: 122.976475ms, avg blocking time: 750.155µs, avg load: 1.0
    coopsched_test.go:148: Avg delay overhead: 485.355479ms, avg running time: 92.898µs, avg waiting time: 127.490722ms, avg blocking time: 461.529222ms, avg load: 73.7
    coopsched_test.go:148: Avg delay overhead: 23.775140596s, avg running time: 113.296µs, avg waiting time: 128.432584ms, avg blocking time: 23.869675127s, avg load: 3464.1
BenchmarkRunningTimeFair/yield/channel-4         	    3610	   6867427 ns/op
BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:180: Avg delay overhead: 24.038897ms, avg running time: 196.19617ms, avg waiting time: 123.267015ms, avg blocking time: 869.139µs, avg load: 1.0
    coopsched_test.go:180: Avg delay overhead: 672.438526ms, avg running time: 123.338141ms, avg waiting time: 128.568946ms, avg blocking time: 5.277720363s, avg load: 165.7
    coopsched_test.go:180: Avg delay overhead: 3.896321999s, avg running time: 122.752475ms, avg waiting time: 127.077907ms, avg blocking time: 27.963958195s, avg load: 916.2
BenchmarkRunningTimeFair/yield/mixed-4           	     504	  47866332 ns/op
BenchmarkRunningTimeFair/noYield
BenchmarkRunningTimeFair/noYield/cpu
    coopsched_test.go:123: Avg running time: 96.982406ms, avg waiting time: 0s, avg blocking time: 2.46µs, avg load: 1.0
    coopsched_test.go:123: Avg running time: 122.110852ms, avg waiting time: 0s, avg blocking time: 1.980627696s, avg load: 50.5
    coopsched_test.go:123: Avg running time: 122.139409ms, avg waiting time: 0s, avg blocking time: 11.843386228s, avg load: 290.0
BenchmarkRunningTimeFair/noYield/cpu-4           	     585	  40724884 ns/op
BenchmarkRunningTimeFair/noYield/channel
    coopsched_test.go:148: Avg delay overhead: 28.269119ms, avg running time: 128.31721ms, avg waiting time: 0s, avg blocking time: 4.94µs, avg load: 1.0
    coopsched_test.go:148: Avg delay overhead: 24.978196ms, avg running time: 125.019909ms, avg waiting time: 0s, avg blocking time: 2.019713943s, avg load: 50.5
    coopsched_test.go:148: Avg delay overhead: 25.575119ms, avg running time: 125.618253ms, avg waiting time: 0s, avg blocking time: 11.733462975s, avg load: 279.5
BenchmarkRunningTimeFair/noYield/channel-4       	     564	  41890761 ns/op
BenchmarkRunningTimeFair/noYield/mixed
    coopsched_test.go:180: Avg delay overhead: 9.222699ms, avg running time: 238.357244ms, avg waiting time: 0s, avg blocking time: 33.846µs, avg load: 1.5
    coopsched_test.go:180: Avg delay overhead: 14.407771ms, avg running time: 227.817446ms, avg waiting time: 0s, avg blocking time: 7.446002598s, avg load: 97.5
    coopsched_test.go:180: Avg delay overhead: 13.883597ms, avg running time: 223.97811ms, avg waiting time: 0s, avg blocking time: 23.500702549s, avg load: 311.1
BenchmarkRunningTimeFair/noYield/mixed-4         	     313	  74731269 ns/op
PASS
ok  	github.com/tommie/go-coopsched	343.508s
```
