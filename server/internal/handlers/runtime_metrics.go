package handlers

import (
	"math"
	"net/http"
	"runtime/metrics"
	"time"
)

// RuntimeMetrics — метрики самого Go-процесса (рантайм приложения), снятые
// через стандартный пакет runtime/metrics. В отличие от системных метрик
// (CPU/RAM/диск/сеть хоста) описывают только наше приложение: куча, GC,
// горутины и планировщик. Не требует внешних зависимостей.
type RuntimeMetrics struct {
	Goroutines         uint64 `json:"goroutines"`
	GoroutinesRunning  uint64 `json:"goroutines_running"`
	GoroutinesRunnable uint64 `json:"goroutines_runnable"`
	Gomaxprocs         uint64 `json:"gomaxprocs"`

	// GC.
	GCCycles        uint64  `json:"gc_cycles"`
	GOGC            uint64  `json:"gogc"`
	NextGCBytes     uint64  `json:"next_gc_bytes"`
	GCCPUSeconds    float64 `json:"gc_cpu_seconds"`
	TotalCPUSeconds float64 `json:"total_cpu_seconds"`
	GCCPUPercent    float64 `json:"gc_cpu_percent"`
	GCPauseCount    uint64  `json:"gc_pause_count"`
	GCPauseSeconds  float64 `json:"gc_pause_seconds"`

	// Память (классы кучи Go, не физическая RAM хоста).
	HeapLiveBytes      uint64 `json:"heap_live_bytes"`
	HeapObjects        uint64 `json:"heap_objects"`
	HeapObjectsBytes   uint64 `json:"heap_objects_bytes"`
	HeapUnusedBytes    uint64 `json:"heap_unused_bytes"`
	HeapStacksBytes    uint64 `json:"heap_stacks_bytes"`
	ProcessMemoryBytes uint64 `json:"process_memory_bytes"`
	AllocBytesTotal    uint64 `json:"alloc_bytes_total"`
	FreeBytesTotal     uint64 `json:"free_bytes_total"`

	// Планировщик: задержки (гистограмма) — количество и суммарное время.
	SchedLatencyCount   uint64  `json:"sched_latency_count"`
	SchedLatencySeconds float64 `json:"sched_latency_seconds"`

	Timestamp string `json:"timestamp"`
}

// runtimeMetricNames — читаемые метрики в фиксированном порядке (индексы
// используются ниже при разборе сэмплов).
var runtimeMetricNames = []string{
	"/sched/goroutines:goroutines",
	"/sched/goroutines/running:goroutines",
	"/sched/goroutines/runnable:goroutines",
	"/sched/gomaxprocs:threads",
	"/gc/cycles/total:gc-cycles",
	"/gc/gogc:percent",
	"/gc/heap/goal:bytes",
	"/gc/heap/live:bytes",
	"/gc/heap/objects:objects",
	"/memory/classes/heap/objects:bytes",
	"/memory/classes/heap/unused:bytes",
	"/memory/classes/heap/stacks:bytes",
	"/memory/classes/total:bytes",
	"/gc/heap/allocs:bytes",
	"/gc/heap/frees:bytes",
	"/cpu/classes/gc/total:cpu-seconds",
	"/cpu/classes/total:cpu-seconds",
	"/gc/pauses:seconds",
	"/sched/latencies:seconds",
}

// CollectRuntimeMetrics снимает метрики рантайма Go одним вызовом metrics.Read.
func CollectRuntimeMetrics() RuntimeMetrics {
	samples := make([]metrics.Sample, len(runtimeMetricNames))
	for i, n := range runtimeMetricNames {
		samples[i].Name = n
	}
	metrics.Read(samples)

	var m RuntimeMetrics
	m.Goroutines = readUint(samples[0])
	m.GoroutinesRunning = readUint(samples[1])
	m.GoroutinesRunnable = readUint(samples[2])
	m.Gomaxprocs = readUint(samples[3])
	m.GCCycles = readUint(samples[4])
	m.GOGC = readUint(samples[5])
	m.NextGCBytes = readUint(samples[6])
	m.HeapLiveBytes = readUint(samples[7])
	m.HeapObjects = readUint(samples[8])
	m.HeapObjectsBytes = readUint(samples[9])
	m.HeapUnusedBytes = readUint(samples[10])
	m.HeapStacksBytes = readUint(samples[11])
	m.ProcessMemoryBytes = readUint(samples[12])
	m.AllocBytesTotal = readUint(samples[13])
	m.FreeBytesTotal = readUint(samples[14])
	m.GCCPUSeconds = readFloat(samples[15])
	m.TotalCPUSeconds = readFloat(samples[16])
	if m.TotalCPUSeconds > 0 {
		m.GCCPUPercent = m.GCCPUSeconds / m.TotalCPUSeconds * 100
	}
	m.GCPauseCount, m.GCPauseSeconds = histStats(samples[17])
	m.SchedLatencyCount, m.SchedLatencySeconds = histStats(samples[18])
	m.Timestamp = time.Now().Format(time.RFC3339)
	return m
}

// readUint возвращает значение сэмпла как uint64 (0 для иного/недоступного типа).
func readUint(s metrics.Sample) uint64 {
	if s.Value.Kind() == metrics.KindUint64 {
		return s.Value.Uint64()
	}
	return 0
}

// readFloat возвращает значение сэмпла как float64 (0 для гистограммы/недоступного).
func readFloat(s metrics.Sample) float64 {
	switch s.Value.Kind() {
	case metrics.KindFloat64:
		return s.Value.Float64()
	case metrics.KindUint64:
		return float64(s.Value.Uint64())
	}
	return 0
}

// histStats оценивает количество наблюдений и сумму по гистограмме:
// для каждого бакета берётся его середина (крайние бесконечные бакеты —
// по конечной границе).
func histStats(s metrics.Sample) (count uint64, total float64) {
	if s.Value.Kind() != metrics.KindFloat64Histogram {
		return 0, 0
	}
	h := s.Value.Float64Histogram()
	for i, c := range h.Counts {
		count += c
		if c == 0 {
			continue
		}
		lo, hi := h.Buckets[i], h.Buckets[i+1]
		switch {
		case math.IsInf(lo, -1) && math.IsInf(hi, 1):
			// единственный бакет — оценку дать нельзя
		case math.IsInf(lo, -1):
			total += float64(c) * hi
		case math.IsInf(hi, 1):
			total += float64(c) * lo
		default:
			total += float64(c) * (lo + hi) / 2
		}
	}
	return count, total
}

// AppMetrics — метрики рантайма приложения (только для администраторов).
func (h *Handlers) AppMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, CollectRuntimeMetrics())
}
