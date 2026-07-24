package server

import (
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	StartedAt time.Time

	Requests      atomic.Uint64
	Errors        atomic.Uint64
	MethodCounts  sync.Map
	TotalDuration atomic.Uint64

	FloodWaits atomic.Uint64
	Auth401s   atomic.Uint64
	Forbidden  atomic.Uint64

	BytesIn  atomic.Uint64
	BytesOut atomic.Uint64
}

var metrics = &Metrics{StartedAt: time.Now()}

type methodStat struct {
	count   atomic.Uint64
	durSum  atomic.Uint64
	errored atomic.Uint64
}

func recordCall(method string, dur time.Duration, err error, bytesIn, bytesOut int) {
	metrics.Requests.Add(1)
	metrics.TotalDuration.Add(uint64(dur.Nanoseconds()))
	metrics.BytesIn.Add(uint64(bytesIn))
	metrics.BytesOut.Add(uint64(bytesOut))

	v, _ := metrics.MethodCounts.LoadOrStore(method, &methodStat{})
	ms := v.(*methodStat)
	ms.count.Add(1)
	ms.durSum.Add(uint64(dur.Nanoseconds()))

	if err != nil {
		metrics.Errors.Add(1)
		ms.errored.Add(1)
	}
}
