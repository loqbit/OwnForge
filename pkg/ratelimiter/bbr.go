package ratelimiter

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

// BBRLimiter 结构体
type BBRLimiter struct {
	// CPU 触发条件
	cpuUsage     int64 // 当前 CPU 使用率（乘100存整数，避免浮点原子操作）
	cpuThreshold int64 // CPU 触发阈值（默认 80，即 80%）

	inflight int64 // 当前正在处理的请求数（原子操作）

	// 滑动窗口采样（存最近 N 个桶的 QPS 和 RT）
	buckets   []bucket      // 环形缓冲区
	bucketNum int           // 桶的数量（比如 100 个）
	window    time.Duration // 整个采样窗口时长（比如 10 秒）

	mu sync.Mutex // 保护 buckets 的写入
}

// bucket 结构体（内部使用）
type bucket struct {
	count   int64         // 这个时间片内完成了多少请求
	rt      time.Duration // 这个时间片内的总 RT（用于求平均）
	startAt time.Time     // 这个桶的起始时间
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

// 启动 CPU 采样器
func (b *BBRLimiter) startCPUSampler() {
	go func() {
		for {
			// gopsutil: 250ms 采样间隔，false 表示取总体 CPU（不按核心拆分）
			percents, err := cpu.Percent(250*time.Millisecond, false)
			if err == nil && len(percents) > 0 {
				atomic.StoreInt64(&b.cpuUsage, int64(percents[0]))
			}
		}
	}()
}

// 获取当前CPU使用率
func (b *BBRLimiter) CPUUsage() int64 { return atomic.LoadInt64(&b.cpuUsage) }

// 增加正在处理的请求数
func (b *BBRLimiter) IncrInflight() { atomic.AddInt64(&b.inflight, 1) }

// 减少正在处理的请求数
func (b *BBRLimiter) DecrInflight() { atomic.AddInt64(&b.inflight, -1) }

// 获取正在处理的请求数
func (b *BBRLimiter) Inflight() int64 { return atomic.LoadInt64(&b.inflight) }

// 滑动窗口计算所有桶的最大QPS和最小RT
func (b *BBRLimiter) MaxFlight() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	var maxQPS float64
	var minRT float64 = math.MaxFloat64
	bucketDur := b.window / time.Duration(b.bucketNum)

	for _, bucket := range b.buckets {
		// 跳过过期桶和空桶
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

// 获取当前桶的索引
func (b *BBRLimiter) currentIndex() int {
	// 计算每个桶的时间长度
	bucketDur := b.window / time.Duration(b.bucketNum)
	// 计算当前时间距离第一个桶的起始时间的毫秒数,除以每个桶的时间长度,得到当前桶的索引
	return int(time.Now().UnixMilli()/bucketDur.Milliseconds()) % b.bucketNum
}

// 记录RT
func (b *BBRLimiter) RecordRT(rt time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 获取当前桶的索引
	idx := b.currentIndex()
	// 获取当前桶
	bucket := &b.buckets[idx]

	// 如果这个桶已经过期了（属于上一轮），重置它
	if time.Since(bucket.startAt) > b.window {
		bucket.count = 0
		bucket.rt = 0
		bucket.startAt = time.Now()
	}

	bucket.count++
	bucket.rt += rt
}

// 判断是否应该拒绝
func (b *BBRLimiter) ShouldReject() bool {
	// 第一关：CPU 低于阈值 → 直接放行，系统没压力
	if b.CPUUsage() < b.cpuThreshold {
		return false
	}
	// 第二关：CPU 高了，进行精确判断
	maxFlight := b.MaxFlight()
	return maxFlight > 0 && float64(b.Inflight()) > maxFlight
}
