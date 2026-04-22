package ratelimiter

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

// BBRLimiter struct
type BBRLimiter struct {
	// CPU trigger conditions
	cpuUsage     int64 // current CPU usage (stored as an integer multiplied by 100 to avoid atomic float operations)
	cpuThreshold int64 // CPU threshold (default 80, meaning 80%)

	inflight int64 // current number of in-flight requests (atomic)

	// sliding-window sampling that stores QPS and RT for the most recent N buckets
	buckets   []bucket      // ring buffer
	bucketNum int           // number of buckets (for example, 100)
	window    time.Duration // total sampling window duration (for example, 10 seconds)

	mu sync.Mutex // protect writes to buckets
}

// bucket struct (internal use)
type bucket struct {
	count   int64         // how many requests completed in this time slice
	rt      time.Duration // total RT in this time slice, used to compute the average
	startAt time.Time     // start time of this bucket
}

func NewBBRLimiter(bucketNum int, window time.Duration, cpuThreshold int64) *BBRLimiter {
	limiter := &BBRLimiter{
		cpuThreshold: cpuThreshold,
		bucketNum:    bucketNum,
		window:       window,
		buckets:      make([]bucket, bucketNum),
	}
	limiter.startCPUSampler()
	return limiter
}

// start the CPU sampler
func (b *BBRLimiter) startCPUSampler() {
	go func() {
		for {
			// gopsutil: 250ms sampling interval; false means overall CPU rather than per-core CPU
			percents, err := cpu.Percent(250*time.Millisecond, false)
			if err == nil && len(percents) > 0 {
				atomic.StoreInt64(&b.cpuUsage, int64(percents[0]))
			}
		}
	}()
}

// get current CPU usage
func (b *BBRLimiter) CPUUsage() int64 { return atomic.LoadInt64(&b.cpuUsage) }

// increase the number of in-flight requests
func (b *BBRLimiter) IncrInflight() { atomic.AddInt64(&b.inflight, 1) }

// decrease the number of in-flight requests
func (b *BBRLimiter) DecrInflight() { atomic.AddInt64(&b.inflight, -1) }

// get the number of in-flight requests
func (b *BBRLimiter) Inflight() int64 { return atomic.LoadInt64(&b.inflight) }

// scan the sliding window to calculate the maximum QPS and minimum RT across all buckets
func (b *BBRLimiter) MaxFlight() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	var maxQPS float64
	var minRT float64 = math.MaxFloat64
	bucketDur := b.window / time.Duration(b.bucketNum)

	for _, bucket := range b.buckets {
		// skip expired buckets and empty buckets
		if time.Since(bucket.startAt) > b.window || bucket.count == 0 {
			continue
		}

		qps := float64(bucket.count) / bucketDur.Seconds()
		avgRT := float64(bucket.rt) / float64(bucket.count)

		if qps > maxQPS {
			maxQPS = qps
		}
		if avgRT < minRT {
			minRT = avgRT
		}
	}

	return maxQPS * (minRT / float64(time.Second))
}

// get the current bucket index
func (b *BBRLimiter) currentIndex() int {
	// compute the duration of each bucket
	bucketDur := b.window / time.Duration(b.bucketNum)
	// compute the milliseconds since the first bucket started and divide by bucket duration to get the current bucket index
	return int(time.Now().UnixMilli()/bucketDur.Milliseconds()) % b.bucketNum
}

// record RT
func (b *BBRLimiter) RecordRT(rt time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// get the current bucket index
	idx := b.currentIndex()
	// get the current bucket
	bucket := &b.buckets[idx]

	// if this bucket has expired from the previous cycle, reset it
	if time.Since(bucket.startAt) > b.window {
		bucket.count = 0
		bucket.rt = 0
		bucket.startAt = time.Now()
	}

	bucket.count++
	bucket.rt += rt
}

// determine whether the request should be rejected
func (b *BBRLimiter) ShouldReject() bool {
	// First gate: if CPU is below the threshold, allow immediately because the system is not under pressure.
	if b.CPUUsage() < b.cpuThreshold {
		return false
	}
	// Second gate: if CPU is high, perform a more precise check.
	maxFlight := b.MaxFlight()
	return maxFlight > 0 && float64(b.Inflight()) > maxFlight
}
