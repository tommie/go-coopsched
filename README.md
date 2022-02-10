# A benchmark and playground for Completely Fair Scheduling in Go

See https://github.com/golang/go/issues/51071. I just wanted to play
around with it myself.

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

## Initial Results (on a ThinkPad T460s laptop)

Second version.

The "avg delay overhead" is how long we waited in `<-time.After`,
after discounting the expected delay. Running time and blocking time
are total times for all scheduled goroutines.

See below for full output. The case we care about is "mixed". No-yield
should all be equivalent, and never blocks in `Yield`. The numbers are
in the same ballpark, but the accuracy isn't very high:

```
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:163: Avg delay overhead: 7.899607ms, running time: 230.484823ms, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 2.96121798s, running time: 11m30.510833596s, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 10.632686262s, running time: 5h44m5.281539223s, blocking time: 0s
BenchmarkFIFO/noYield/mixed-4                616          37750453 ns/op

BenchmarkRunningTimeFair/noYield/mixed
    coopsched_test.go:163: Avg delay overhead: 8.929726ms, running time: 213.570909ms, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 3.337315731s, running time: 12m49.200893796s, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 8.083276872s, running time: 4h49m25.899732574s, blocking time: 0s
BenchmarkRunningTimeFair/noYield/mixed-4                     571          39737839 ns/op
```

FIFO is what Go does, so we wouldn't expect that to be very different
from no-yield, except for more overhead. In fact, it seems to improve
the overhead. Though, there's more time spent blocking, which
increases the overall time:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:163: Avg delay overhead: 8.983948ms, running time: 213.889263ms, blocking time: 56.197µs
    coopsched_test.go:163: Avg delay overhead: 1.225087519s, running time: 4m41.351820796s, blocking time: 6m29.435407365s
    coopsched_test.go:163: Avg delay overhead: 10.377686903s, running time: 3h50m10.987829141s, blocking time: 4h10m12.566186525s
BenchmarkFIFO/yield/mixed-4                  637          37428497 ns/op

BenchmarkFIFO/noYield/mixed
    coopsched_test.go:163: Avg delay overhead: 7.899607ms, running time: 230.484823ms, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 2.96121798s, running time: 11m30.510833596s, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 10.632686262s, running time: 5h44m5.281539223s, blocking time: 0s
BenchmarkFIFO/noYield/mixed-4                616          37750453 ns/op
```

Finally, RunningTimeFair should give a lower overhead than FIFO, but
it didn't. The data is very noisy:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:163: Avg delay overhead: 8.983948ms, running time: 213.889263ms, blocking time: 56.197µs
    coopsched_test.go:163: Avg delay overhead: 1.225087519s, running time: 4m41.351820796s, blocking time: 6m29.435407365s
    coopsched_test.go:163: Avg delay overhead: 10.377686903s, running time: 3h50m10.987829141s, blocking time: 4h10m12.566186525s
BenchmarkFIFO/yield/mixed-4                  637          37428497 ns/op

BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:163: Avg delay overhead: 9.481622ms, running time: 222.212984ms, blocking time: 49.651µs
    coopsched_test.go:163: Avg delay overhead: 1.447611353s, running time: 4m26.331438834s, blocking time: 7m24.435948978s
    coopsched_test.go:163: Avg delay overhead: 11.518185357s, running time: 4h4m22.4169432s, blocking time: 3h32m4.558693558s
BenchmarkRunningTimeFair/yield/mixed-4                       614          38278115 ns/op
```

## Conclusions

None, yet. The sanity checks worked out, but that may have been
noise. The actually interesting case isn't conclusive, even after
running more goroutines.

## Output from Initial Benchmarking

```console
$ go test -v -bench . -benchtime 20s ./
goos: linux
goarch: amd64
pkg: github.com/tommie/go-coopsched
cpu: Intel(R) Core(TM) i7-6600U CPU @ 2.60GHz
BenchmarkFIFO
BenchmarkFIFO/yield
BenchmarkFIFO/yield/cpu
    coopsched_test.go:110: Running time: 86.946623ms, blocking time: 38.864µs
    coopsched_test.go:110: Running time: 2m28.089645471s, blocking time: 2m4.314626941s
    coopsched_test.go:110: Running time: 3h19m20.214566986s, blocking time: 56m8.2801993s
BenchmarkFIFO/yield/cpu-4            698          34471889 ns/op
BenchmarkFIFO/yield/channel
    coopsched_test.go:133: Avg delay overhead: 28.271149ms, running time: 128.34127ms, blocking time: 57.086µs
    coopsched_test.go:133: Avg delay overhead: 36.64858ms, running time: 13.68364071s, blocking time: 61.206613ms
    coopsched_test.go:133: Avg delay overhead: 235.447478ms, running time: 58m6.491130583s, blocking time: 1h30m24.612176725s
    coopsched_test.go:133: Avg delay overhead: 209.512268ms, running time: 20h25m3.356534681s, blocking time: 1037h26m8.880249289s
BenchmarkFIFO/yield/channel-4             226012             95757 ns/op
BenchmarkFIFO/yield/mixed
    coopsched_test.go:163: Avg delay overhead: 8.983948ms, running time: 213.889263ms, blocking time: 56.197µs
    coopsched_test.go:163: Avg delay overhead: 1.225087519s, running time: 4m41.351820796s, blocking time: 6m29.435407365s
    coopsched_test.go:163: Avg delay overhead: 10.377686903s, running time: 3h50m10.987829141s, blocking time: 4h10m12.566186525s
BenchmarkFIFO/yield/mixed-4                  637          37428497 ns/op
BenchmarkFIFO/noYield
BenchmarkFIFO/noYield/cpu
    coopsched_test.go:110: Running time: 94.264959ms, blocking time: 0s
    coopsched_test.go:110: Running time: 4m44.17775338s, blocking time: 0s
    coopsched_test.go:110: Running time: 3h58m27.047245961s, blocking time: 0s
BenchmarkFIFO/noYield/cpu-4                  646          37147051 ns/op
BenchmarkFIFO/noYield/channel
    coopsched_test.go:133: Avg delay overhead: 33.039286ms, running time: 133.079526ms, blocking time: 0s
    coopsched_test.go:133: Avg delay overhead: 32.49714ms, running time: 13.257884279s, blocking time: 0s
    coopsched_test.go:133: Avg delay overhead: 161.778659ms, running time: 59m16.450454483s, blocking time: 0s
    coopsched_test.go:133: Avg delay overhead: 703.27576ms, running time: 149h42m15.841018779s, blocking time: 0s
BenchmarkFIFO/noYield/channel-4           395838             53624 ns/op
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:163: Avg delay overhead: 7.899607ms, running time: 230.484823ms, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 2.96121798s, running time: 11m30.510833596s, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 10.632686262s, running time: 5h44m5.281539223s, blocking time: 0s
BenchmarkFIFO/noYield/mixed-4                616          37750453 ns/op
BenchmarkRunningTimeFair
BenchmarkRunningTimeFair/yield
BenchmarkRunningTimeFair/yield/cpu
    coopsched_test.go:110: Running time: 97.200937ms, blocking time: 18.64µs
    coopsched_test.go:110: Running time: 5m14.497097433s, blocking time: 27.421390982s
    coopsched_test.go:110: Running time: 2h36m10.115690868s, blocking time: 1h22m30.232122156s
BenchmarkRunningTimeFair/yield/cpu-4         639          37910335 ns/op
BenchmarkRunningTimeFair/yield/channel
    coopsched_test.go:133: Avg delay overhead: 29.836695ms, running time: 129.917518ms, blocking time: 76.397µs
    coopsched_test.go:133: Avg delay overhead: 36.308827ms, running time: 13.637967016s, blocking time: 73.599916ms
    coopsched_test.go:133: Avg delay overhead: 122.665892ms, running time: 38m15.580186686s, blocking time: 50m4.081360782s
    coopsched_test.go:133: Avg delay overhead: 127.486286ms, running time: 21h59m14.003817582s, blocking time: 2633h46m56.506012235s
BenchmarkRunningTimeFair/yield/channel-4                  332668            105224 ns/op
BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:163: Avg delay overhead: 9.481622ms, running time: 222.212984ms, blocking time: 49.651µs
    coopsched_test.go:163: Avg delay overhead: 1.447611353s, running time: 4m26.331438834s, blocking time: 7m24.435948978s
    coopsched_test.go:163: Avg delay overhead: 11.518185357s, running time: 4h4m22.4169432s, blocking time: 3h32m4.558693558s
BenchmarkRunningTimeFair/yield/mixed-4                       614          38278115 ns/op
BenchmarkRunningTimeFair/noYield
BenchmarkRunningTimeFair/noYield/cpu
    coopsched_test.go:110: Running time: 97.588847ms, blocking time: 0s
    coopsched_test.go:110: Running time: 5m50.025869318s, blocking time: 0s
    coopsched_test.go:110: Running time: 3h59m15.359780468s, blocking time: 0s
BenchmarkRunningTimeFair/noYield/cpu-4                       639          37756932 ns/op
BenchmarkRunningTimeFair/noYield/channel
    coopsched_test.go:133: Avg delay overhead: 28.031024ms, running time: 128.064252ms, blocking time: 0s
    coopsched_test.go:133: Avg delay overhead: 28.155254ms, running time: 12.823550472s, blocking time: 0s
    coopsched_test.go:133: Avg delay overhead: 185.333688ms, running time: 58m11.921194291s, blocking time: 0s
    coopsched_test.go:133: Avg delay overhead: 988.315354ms, running time: 193h51m46.787625771s, blocking time: 0s
BenchmarkRunningTimeFair/noYield/channel-4                420523             55109 ns/op
BenchmarkRunningTimeFair/noYield/mixed
    coopsched_test.go:163: Avg delay overhead: 8.929726ms, running time: 213.570909ms, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 3.337315731s, running time: 12m49.200893796s, blocking time: 0s
    coopsched_test.go:163: Avg delay overhead: 8.083276872s, running time: 4h49m25.899732574s, blocking time: 0s
BenchmarkRunningTimeFair/noYield/mixed-4                     571          39737839 ns/op
PASS
ok      github.com/tommie/go-coopsched  329.202s
```
