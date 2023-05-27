# contextcond

[![Go Reference](https://pkg.go.dev/badge/github.com/Jille/contextcond.svg)](https://pkg.go.dev/github.com/Jille/contextcond)

This library provides a [sync.Cond](https://pkg.go.dev/sync#Cond) that has a `WaitContext()` method.

`WaitContext(context.Context)` waits for either the condvar to wake it, or the context to be cancelled.
It returns nil if the condvar woke it, or ctx.Err() if the context was cancelled.

## Benchmark

contextcond is slower than sync.Cond, so you should only use it if you need `WaitContext()`.

```
$ go test -bench .
goos: linux
goarch: amd64
pkg: github.com/Jille/contextcond
cpu: Intel(R) Xeon(R) CPU E5-2650 0 @ 2.00GHz
BenchmarkCond1-8          560541              1968 ns/op
BenchmarkCond2-8          196060              7049 ns/op
BenchmarkCond4-8           95750             12685 ns/op
BenchmarkCond8-8           49628             25710 ns/op
BenchmarkCond16-8          34155             37504 ns/op
BenchmarkCond32-8          14602             82923 ns/op
```

Compared to sync.Cond:

```
$ go test -bench . /usr/lib/go-1.19/src/sync/cond_test.go
goos: linux
goarch: amd64
cpu: Intel(R) Xeon(R) CPU E5-2650 0 @ 2.00GHz
BenchmarkCond1-8         1509849               782.1 ns/op
BenchmarkCond2-8          447687              2589 ns/op
BenchmarkCond4-8          251739              5226 ns/op
BenchmarkCond8-8          118968             11194 ns/op
BenchmarkCond16-8          44931             24098 ns/op
BenchmarkCond32-8          28124             40405 ns/op
```
