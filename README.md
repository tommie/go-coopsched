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
    coopsched_test.go:165: Avg delay overhead: 9.901168ms, avg running time: 232.950486ms, avg blocking time: 18.101µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 10.124151ms, avg running time: 216.68899ms, avg blocking time: 7.179328681s, avg load: 100.5
    coopsched_test.go:165: Avg delay overhead: 10.331689ms, avg running time: 217.711998ms, avg blocking time: 24.029724327s, avg load: 329.7
BenchmarkFIFO/noYield/mixed-4         	     330	  72640885 ns/op

BenchmarkRunningTimeFair/noYield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.604123ms, avg running time: 237.221742ms, avg blocking time: 24.927µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 13.491506ms, avg running time: 223.819904ms, avg blocking time: 7.434827692s, avg load: 100.5
    coopsched_test.go:165: Avg delay overhead: 15.789219ms, avg running time: 228.933039ms, avg blocking time: 24.521774098s, avg load: 320.2
BenchmarkRunningTimeFair/noYield/mixed-4         	     320	  76478345 ns/op
```

FIFO is what Go does, so we wouldn't expect that to be very different
from no-yield, except for more overhead. In fact, it seems to improve
the overhead. That overhead shows up as more blocking time:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.386379ms, avg running time: 232.744535ms, avg blocking time: 104.194µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 15.01817ms, avg running time: 223.416672ms, avg blocking time: 14.103694027s, avg load: 183.0
    coopsched_test.go:165: Avg delay overhead: 10.96578ms, avg running time: 219.09822ms, avg blocking time: 44.479651716s, avg load: 586.2
BenchmarkFIFO/yield/mixed-4           	     321	  73030181 ns/op

BenchmarkFIFO/noYield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.901168ms, avg running time: 232.950486ms, avg blocking time: 18.101µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 10.124151ms, avg running time: 216.68899ms, avg blocking time: 7.179328681s, avg load: 100.5
    coopsched_test.go:165: Avg delay overhead: 10.331689ms, avg running time: 217.711998ms, avg blocking time: 24.029724327s, avg load: 329.7
BenchmarkFIFO/noYield/mixed-4         	     330	  72640885 ns/op
```

Finally, RunningTimeFair should give a lower overhead than FIFO, but
it didn't. The data is noisy, but it's the highest numbers of any
"mixed":

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.386379ms, avg running time: 232.744535ms, avg blocking time: 104.194µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 15.01817ms, avg running time: 223.416672ms, avg blocking time: 14.103694027s, avg load: 183.0
    coopsched_test.go:165: Avg delay overhead: 10.96578ms, avg running time: 219.09822ms, avg blocking time: 44.479651716s, avg load: 586.2
BenchmarkFIFO/yield/mixed-4           	     321	  73030181 ns/op

BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.455671ms, avg running time: 237.943263ms, avg blocking time: 102.989µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 19.706727ms, avg running time: 243.623092ms, avg blocking time: 15.154097683s, avg load: 180.8
    coopsched_test.go:165: Avg delay overhead: 18.468097ms, avg running time: 236.63098ms, avg blocking time: 44.224240369s, avg load: 540.6
BenchmarkRunningTimeFair/yield/mixed-4           	     294	  78965625 ns/op
```

## Conclusions

None, yet. The sanity checks worked out, but that may have been
noise. The actually interesting case isn't conclusive, even after
running more goroutines.

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

Third version.

```console
$ go test -v -bench . -benchtime 20s ./
goos: linux
goarch: amd64
pkg: github.com/tommie/go-coopsched
cpu: Intel(R) Core(TM) i7-6600U CPU @ 2.60GHz
BenchmarkFIFO
BenchmarkFIFO/yield
BenchmarkFIFO/yield/cpu
    coopsched_test.go:110: Avg running time: 91.379328ms, avg blocking time: 50.24µs, avg load: 1.0
    coopsched_test.go:110: Avg running time: 115.295986ms, avg blocking time: 3.576714355s, avg load: 90.9
    coopsched_test.go:110: Avg running time: 116.832355ms, avg blocking time: 22.980571187s, avg load: 572.5
BenchmarkFIFO/yield/cpu-4    	     622	  38728367 ns/op
BenchmarkFIFO/yield/channel
    coopsched_test.go:134: Avg delay overhead: 22.192481ms, avg running time: 122.342331ms, avg blocking time: 88.597µs, avg load: 1.0
    coopsched_test.go:134: Avg delay overhead: 23.534918ms, avg running time: 123.647689ms, avg blocking time: 3.914607065s, avg load: 92.5
    coopsched_test.go:134: Avg delay overhead: 24.315748ms, avg running time: 124.458974ms, avg blocking time: 23.476679294s, avg load: 546.0
BenchmarkFIFO/yield/channel-4         	     580	  41518097 ns/op
BenchmarkFIFO/yield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.386379ms, avg running time: 232.744535ms, avg blocking time: 104.194µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 15.01817ms, avg running time: 223.416672ms, avg blocking time: 14.103694027s, avg load: 183.0
    coopsched_test.go:165: Avg delay overhead: 10.96578ms, avg running time: 219.09822ms, avg blocking time: 44.479651716s, avg load: 586.2
BenchmarkFIFO/yield/mixed-4           	     321	  73030181 ns/op
BenchmarkFIFO/noYield
BenchmarkFIFO/noYield/cpu
    coopsched_test.go:110: Avg running time: 97.320038ms, avg blocking time: 5.782µs, avg load: 1.0
    coopsched_test.go:110: Avg running time: 121.012584ms, avg blocking time: 1.953039661s, avg load: 50.5
    coopsched_test.go:110: Avg running time: 122.253636ms, avg blocking time: 11.908926089s, avg load: 295.0
BenchmarkFIFO/noYield/cpu-4           	     589	  40811271 ns/op
BenchmarkFIFO/noYield/channel
    coopsched_test.go:134: Avg delay overhead: 28.87019ms, avg running time: 128.904677ms, avg blocking time: 4.746µs, avg load: 1.0
    coopsched_test.go:134: Avg delay overhead: 25.498365ms, avg running time: 125.583127ms, avg blocking time: 2.037190808s, avg load: 50.5
    coopsched_test.go:134: Avg delay overhead: 25.206334ms, avg running time: 125.307101ms, avg blocking time: 11.645912732s, avg load: 280.9
BenchmarkFIFO/noYield/channel-4       	     561	  41763206 ns/op
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.901168ms, avg running time: 232.950486ms, avg blocking time: 18.101µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 10.124151ms, avg running time: 216.68899ms, avg blocking time: 7.179328681s, avg load: 100.5
    coopsched_test.go:165: Avg delay overhead: 10.331689ms, avg running time: 217.711998ms, avg blocking time: 24.029724327s, avg load: 329.7
BenchmarkFIFO/noYield/mixed-4         	     330	  72640885 ns/op
BenchmarkRunningTimeFair
BenchmarkRunningTimeFair/yield
BenchmarkRunningTimeFair/yield/cpu
    coopsched_test.go:110: Avg running time: 98.316433ms, avg blocking time: 67.711µs, avg load: 1.0
    coopsched_test.go:110: Avg running time: 120.851269ms, avg blocking time: 3.762758771s, avg load: 91.2
    coopsched_test.go:110: Avg running time: 122.783922ms, avg blocking time: 23.394930876s, avg load: 550.8
BenchmarkRunningTimeFair/yield/cpu-4  	     594	  41049491 ns/op
BenchmarkRunningTimeFair/yield/channel
    coopsched_test.go:134: Avg delay overhead: 23.142906ms, avg running time: 123.213963ms, avg blocking time: 54.773µs, avg load: 1.0
    coopsched_test.go:134: Avg delay overhead: 24.25845ms, avg running time: 124.374423ms, avg blocking time: 3.970134956s, avg load: 93.2
    coopsched_test.go:134: Avg delay overhead: 24.532348ms, avg running time: 124.721667ms, avg blocking time: 23.582067815s, avg load: 547.4
BenchmarkRunningTimeFair/yield/channel-4         	     577	  41597051 ns/op
BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.455671ms, avg running time: 237.943263ms, avg blocking time: 102.989µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 19.706727ms, avg running time: 243.623092ms, avg blocking time: 15.154097683s, avg load: 180.8
    coopsched_test.go:165: Avg delay overhead: 18.468097ms, avg running time: 236.63098ms, avg blocking time: 44.224240369s, avg load: 540.6
BenchmarkRunningTimeFair/yield/mixed-4           	     294	  78965625 ns/op
BenchmarkRunningTimeFair/noYield
BenchmarkRunningTimeFair/noYield/cpu
    coopsched_test.go:110: Avg running time: 94.57712ms, avg blocking time: 3.466µs, avg load: 1.0
    coopsched_test.go:110: Avg running time: 120.115805ms, avg blocking time: 1.944050108s, avg load: 50.5
    coopsched_test.go:110: Avg running time: 122.311512ms, avg blocking time: 12.040899873s, avg load: 298.5
BenchmarkRunningTimeFair/noYield/cpu-4           	     596	  40755899 ns/op
BenchmarkRunningTimeFair/noYield/channel
    coopsched_test.go:134: Avg delay overhead: 32.154811ms, avg running time: 132.186302ms, avg blocking time: 3.041µs, avg load: 1.0
    coopsched_test.go:134: Avg delay overhead: 26.282835ms, avg running time: 126.387259ms, avg blocking time: 2.047698609s, avg load: 50.5
    coopsched_test.go:134: Avg delay overhead: 25.031885ms, avg running time: 125.119726ms, avg blocking time: 11.629225468s, avg load: 279.6
BenchmarkRunningTimeFair/noYield/channel-4       	     559	  41824446 ns/op
BenchmarkRunningTimeFair/noYield/mixed
    coopsched_test.go:165: Avg delay overhead: 9.604123ms, avg running time: 237.221742ms, avg blocking time: 24.927µs, avg load: 1.0
    coopsched_test.go:165: Avg delay overhead: 13.491506ms, avg running time: 223.819904ms, avg blocking time: 7.434827692s, avg load: 100.5
    coopsched_test.go:165: Avg delay overhead: 15.789219ms, avg running time: 228.933039ms, avg blocking time: 24.521774098s, avg load: 320.2
BenchmarkRunningTimeFair/noYield/mixed-4         	     320	  76478345 ns/op
PASS
ok  	github.com/tommie/go-coopsched	351.362s
```
