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

The "avg delay overhead" is how long we waited in `<-time.After`,
after discounting the expected delay. Running time and blocking time
are total times for all scheduled goroutines.

```console
goos: linux
goarch: amd64
pkg: github.com/tommie/go-coopsched
cpu: Intel(R) Core(TM) i7-6600U CPU @ 2.60GHz
BenchmarkFIFO
BenchmarkFIFO/yield
BenchmarkFIFO/yield/cpu
BenchmarkFIFO/yield/cpu-4     	      33	  35121039 ns/op
BenchmarkFIFO/yield/channel
    coopsched_test.go:127: Avg delay overhead: 25.53852ms, running time: 125.636612ms, blocking time: 110.04µs
    coopsched_test.go:127: Avg delay overhead: 26.0481ms, running time: 1.009083704s, blocking time: 651.076µs
    coopsched_test.go:127: Avg delay overhead: 29.546909ms, running time: 9.595926806s, blocking time: 45.683912ms
    coopsched_test.go:127: Avg delay overhead: 67.864464ms, running time: 1m53.557380473s, blocking time: 3.644420506s
    coopsched_test.go:127: Avg delay overhead: 199.880641ms, running time: 23m15.029016273s, blocking time: 2m48.287793895s
    coopsched_test.go:127: Avg delay overhead: 642.901965ms, running time: 2h34m31.125290211s, blocking time: 11m32.615130378s
BenchmarkFIFO/yield/channel-4 	   11748	     92483 ns/op
BenchmarkFIFO/yield/mixed
    coopsched_test.go:154: Avg delay overhead: 10.932238ms, running time: 283.416783ms, blocking time: 1.40414ms
    coopsched_test.go:154: Avg delay overhead: 110.07767ms, running time: 2.067409778s, blocking time: 303.703713ms
    coopsched_test.go:154: Avg delay overhead: 665.365613ms, running time: 28.508959203s, blocking time: 14.50087039s
BenchmarkFIFO/yield/mixed-4   	      30	  35870821 ns/op
BenchmarkFIFO/noYield
BenchmarkFIFO/noYield/cpu
BenchmarkFIFO/noYield/cpu-4   	      33	  35306826 ns/op
BenchmarkFIFO/noYield/channel
    coopsched_test.go:127: Avg delay overhead: 33.404518ms, running time: 133.433282ms, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 25.447705ms, running time: 1.004011957s, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 30.629034ms, running time: 9.803851212s, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 90.500133ms, running time: 2m10.026069593s, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 138.939375ms, running time: 19m19.815076033s, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 596.254993ms, running time: 2h58m39.853356249s, blocking time: 0s
BenchmarkFIFO/noYield/channel-4         	   13640	     77430 ns/op
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:154: Avg delay overhead: 10.828877ms, running time: 273.887219ms, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 251.70083ms, running time: 4.145260397s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 748.523638ms, running time: 38.605958641s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 936.176552ms, running time: 53.982451825s, blocking time: 0s
BenchmarkFIFO/noYield/mixed-4           	      26	  43325858 ns/op
BenchmarkRunningTimeFair
BenchmarkRunningTimeFair/yield
BenchmarkRunningTimeFair/yield/cpu
BenchmarkRunningTimeFair/yield/cpu-4                 	      33	  35718027 ns/op
BenchmarkRunningTimeFair/yield/channel
    coopsched_test.go:127: Avg delay overhead: 31.031676ms, running time: 131.113225ms, blocking time: 69.307µs
    coopsched_test.go:127: Avg delay overhead: 25.16686ms, running time: 1.002089825s, blocking time: 738.437µs
    coopsched_test.go:127: Avg delay overhead: 32.813733ms, running time: 9.970520461s, blocking time: 49.923033ms
    coopsched_test.go:127: Avg delay overhead: 86.404305ms, running time: 2m5.383249187s, blocking time: 3.033976138s
    coopsched_test.go:127: Avg delay overhead: 116.635443ms, running time: 15m40.176001454s, blocking time: 3m12.676845803s
    coopsched_test.go:127: Avg delay overhead: 578.133296ms, running time: 2h33m48.057423899s, blocking time: 1m37.220618977s
BenchmarkRunningTimeFair/yield/channel-4             	   12486	     82730 ns/op
BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:154: Avg delay overhead: 10.65119ms, running time: 298.474912ms, blocking time: 3.016987ms
    coopsched_test.go:154: Avg delay overhead: 88.487882ms, running time: 1.894642119s, blocking time: 300.890479ms
    coopsched_test.go:154: Avg delay overhead: 429.466775ms, running time: 19.534019216s, blocking time: 12.693080879s
BenchmarkRunningTimeFair/yield/mixed-4               	      28	  37051115 ns/op
BenchmarkRunningTimeFair/noYield
BenchmarkRunningTimeFair/noYield/cpu
BenchmarkRunningTimeFair/noYield/cpu-4               	      32	  35797602 ns/op
BenchmarkRunningTimeFair/noYield/channel
    coopsched_test.go:127: Avg delay overhead: 32.643278ms, running time: 132.684126ms, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 26.209772ms, running time: 1.010164347s, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 33.342007ms, running time: 10.004361833s, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 106.678798ms, running time: 2m17.741560302s, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 112.252885ms, running time: 16m24.556199247s, blocking time: 0s
    coopsched_test.go:127: Avg delay overhead: 643.701124ms, running time: 2h57m8.848265973s, blocking time: 0s
BenchmarkRunningTimeFair/noYield/channel-4           	   12946	     81897 ns/op
BenchmarkRunningTimeFair/noYield/mixed
    coopsched_test.go:154: Avg delay overhead: 11.256915ms, running time: 281.80992ms, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 211.115467ms, running time: 3.116244842s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 779.720172ms, running time: 36.729956378s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 874.972226ms, running time: 50.234275875s, blocking time: 0s
BenchmarkRunningTimeFair/noYield/mixed-4             	      25	  43940464 ns/op
PASS
ok  	github.com/tommie/go-coopsched	24.142s
```

The case we care about is "mixed". No-yield should all be equivalent,
and never blocks in `Yield`. The numbers are in the same ballpark, but
the accuracy isn't very high:

```
BenchmarkFIFO/noYield/mixed
    coopsched_test.go:154: Avg delay overhead: 10.828877ms, running time: 273.887219ms, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 251.70083ms, running time: 4.145260397s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 748.523638ms, running time: 38.605958641s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 936.176552ms, running time: 53.982451825s, blocking time: 0s
BenchmarkFIFO/noYield/mixed-4           	      26	  43325858 ns/op

BenchmarkRunningTimeFair/noYield/mixed
    coopsched_test.go:154: Avg delay overhead: 11.256915ms, running time: 281.80992ms, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 211.115467ms, running time: 3.116244842s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 779.720172ms, running time: 36.729956378s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 874.972226ms, running time: 50.234275875s, blocking time: 0s
BenchmarkRunningTimeFair/noYield/mixed-4             	      25	  43940464 ns/op
```

FIFO is what Go does, so we wouldn't expect that to be very different
from no-yield, except for more overhead. In fact, it seems to improve
the overhead. Though, there's more time spent blocking, which
increases the overall time:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:154: Avg delay overhead: 10.932238ms, running time: 283.416783ms, blocking time: 1.40414ms
    coopsched_test.go:154: Avg delay overhead: 110.07767ms, running time: 2.067409778s, blocking time: 303.703713ms
    coopsched_test.go:154: Avg delay overhead: 665.365613ms, running time: 28.508959203s, blocking time: 14.50087039s
BenchmarkFIFO/yield/mixed-4   	      30	  35870821 ns/op

BenchmarkFIFO/noYield/mixed
    coopsched_test.go:154: Avg delay overhead: 10.828877ms, running time: 273.887219ms, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 251.70083ms, running time: 4.145260397s, blocking time: 0s
    coopsched_test.go:154: Avg delay overhead: 748.523638ms, running time: 38.605958641s, blocking time: 0s
BenchmarkFIFO/noYield/mixed-4           	      26	  43325858 ns/op
```

Finally, RunningTimeFair should give a lower overhead than FIFO:

```
BenchmarkFIFO/yield/mixed
    coopsched_test.go:154: Avg delay overhead: 10.932238ms, running time: 283.416783ms, blocking time: 1.40414ms
    coopsched_test.go:154: Avg delay overhead: 110.07767ms, running time: 2.067409778s, blocking time: 303.703713ms
    coopsched_test.go:154: Avg delay overhead: 665.365613ms, running time: 28.508959203s, blocking time: 14.50087039s
BenchmarkFIFO/yield/mixed-4   	      30	  35870821 ns/op

BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:154: Avg delay overhead: 10.65119ms, running time: 298.474912ms, blocking time: 3.016987ms
    coopsched_test.go:154: Avg delay overhead: 88.487882ms, running time: 1.894642119s, blocking time: 300.890479ms
    coopsched_test.go:154: Avg delay overhead: 429.466775ms, running time: 19.534019216s, blocking time: 12.693080879s
BenchmarkRunningTimeFair/yield/mixed-4               	      28	  37051115 ns/op
```

This first run looked good, but it's actually noise and inconclusive
between runs. Increasing the benchmark size improves it, but it's
still not obviously better:

```console
$ go test -v -bench Benchmark.\*/yield/mixed -benchtime 10s ./
...
BenchmarkFIFO/yield/mixed
    coopsched_test.go:159: Avg delay overhead: 9.287753ms, running time: 201.069855ms, blocking time: 1.364778ms
    coopsched_test.go:159: Avg delay overhead: 572.582867ms, running time: 2m21.811946329s, blocking time: 2m35.642452045s
    coopsched_test.go:159: Avg delay overhead: 6.70111415s, running time: 1h37m10.952363881s, blocking time: 6m18.378755426s
BenchmarkFIFO/yield/mixed-4          342          35406986 ns/op

BenchmarkRunningTimeFair/yield/mixed
    coopsched_test.go:159: Avg delay overhead: 9.059545ms, running time: 293.286788ms, blocking time: 1.480347ms
    coopsched_test.go:159: Avg delay overhead: 751.137261ms, running time: 1m14.694770268s, blocking time: 1m7.05411281s
    coopsched_test.go:159: Avg delay overhead: 6.212873523s, running time: 1h8m50.157255664s, blocking time: 22m40.931335908s
BenchmarkRunningTimeFair/yield/mixed-4               325          36676781 ns/op
PASS
ok      github.com/tommie/go-coopsched  30.163s
```

## Conclusions

None, yet. The sanity checks worked out, but that may have been
noise. The actually interesting case isn't conclusive, even after
running more goroutines.
