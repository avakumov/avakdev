package main

import (
	"bufio"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ServerMetrics — системные метрики сервера, возвращаемые через /api/metrics.
type ServerMetrics struct {
	// CPU — загрузка процессора в процентах [0..100].
	CPU float64 `json:"cpu"`
	// CPUUsedCores — сколько ядер занято (доля).
	CPUUsedCores float64 `json:"cpu_used_cores"`
	// CPUCores — общее количество ядер процессора.
	CPUCores int `json:"cpu_cores"`
	// Memory — использованная физическая память в процентах [0..100].
	Memory float64 `json:"memory"`
	// MemoryUsed — использованная физическая память в байтах.
	MemoryUsed uint64 `json:"memory_used_bytes"`
	// MemoryTotal — вся физическая память в байтах.
	MemoryTotal uint64 `json:"memory_total_bytes"`
	// Disk — заполненность корневого раздела в процентах [0..100].
	Disk float64 `json:"disk"`
	// DiskUsed — использовано на корневом разделе в байтах.
	DiskUsed uint64 `json:"disk_used_bytes"`
	// DiskTotal — всего на корневом разделе в байтах.
	DiskTotal uint64 `json:"disk_total_bytes"`
	// NetworkUp — доступность сети (успешное соединение с внешним хостом).
	NetworkUp bool `json:"network_up"`
	// Network — суммарный сетевой трафик по всем интерфейсам.
	Network NetworkStats `json:"network"`
	// UptimeSeconds — время работы системы с момента загрузки.
	UptimeSeconds int64 `json:"uptime_seconds"`
	// Timestamp — время снятия метрик (RFC3339).
	Timestamp string `json:"timestamp"`
}

// NetworkStats — суммарный сетевой трафик.
type NetworkStats struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

// collectMetrics снимает все системные метрики. Для точности загрузки CPU
// делаются два замера /proc/stat с небольшим интервалом.
func collectMetrics() ServerMetrics {
	cpu, cpuUsedCores, cpuCores := cpuUsage(time.Millisecond * 250)
	memUsed, memTotal, memPct := memoryInfo()
	diskUsed, diskTotal, diskPct := diskInfo("/")
	netUp := networkReachable("1.1.1.1:53", 3*time.Second)
	rx, tx := netTraffic()
	return ServerMetrics{
		CPU:           cpu,
		CPUUsedCores:  cpuUsedCores,
		CPUCores:      cpuCores,
		Memory:        memPct,
		MemoryUsed:    memUsed,
		MemoryTotal:   memTotal,
		Disk:          diskPct,
		DiskUsed:      diskUsed,
		DiskTotal:     diskTotal,
		NetworkUp:     netUp,
		Network:       NetworkStats{RxBytes: rx, TxBytes: tx},
		UptimeSeconds: uptime(),
		Timestamp:     time.Now().Format(time.RFC3339),
	}
}

// cpuUsage возвращает загрузку CPU в процентах и абсолютное значение:
// занятые ядра (доля от общего числа) и общее число ядер.
func cpuUsage(interval time.Duration) (percent, usedCores float64, cores int) {
	cores = runtime.NumCPU()
	idle1, total1, ok1 := readCPUStat()
	time.Sleep(interval)
	idle2, total2, ok2 := readCPUStat()
	if !ok1 || !ok2 || total2 == total1 {
		return 0, 0, cores
	}
	idleDelta := idle2 - idle1
	totalDelta := total2 - total1
	if totalDelta == 0 {
		return 0, 0, cores
	}
	busy := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if busy < 0 {
		busy = 0
	}
	if busy > 100 {
		busy = 100
	}
	if cores > 0 {
		usedCores = busy / 100 * float64(cores)
	} else {
		usedCores = busy
	}
	return busy, usedCores, cores
}

// readCPUStat читает агрегированное значение idle/total из /proc/stat.
func readCPUStat() (idle, total uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return 0, 0, false
		}
		var sums [7]uint64
		for i := 0; i < 7 && i+1 < len(fields); i++ {
			sums[i], _ = strconv.ParseUint(fields[i+1], 10, 64)
		}
		// idle = idle (4) + iowait (5)
		idleVal := sums[3] + sums[4]
		var totalVal uint64
		for _, v := range sums {
			totalVal += v
		}
		return idleVal, totalVal, true
	}
	return 0, 0, false
}

// memoryInfo возвращает использованную/всю память в байтах и процент.
// Значения берутся в килобайтах и переводятся в байты.
func memoryInfo() (usedBytes, totalBytes uint64, percent float64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}
	defer f.Close()

	var memTotalKB, memAvailableKB uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			memTotalKB = val
		case "MemAvailable:":
			memAvailableKB = val
		}
	}
	if memTotalKB == 0 {
		return 0, 0, 0
	}
	usedKB := memTotalKB - memAvailableKB
	percent = float64(usedKB) / float64(memTotalKB) * 100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return usedKB * 1024, memTotalKB * 1024, percent
}

// diskInfo возвращает использованное/всего место на разделе в байтах и процент.
func diskInfo(path string) (usedBytes, totalBytes uint64, percent float64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	if total == 0 {
		return 0, 0, 0
	}
	used := total - free
	percent = float64(used) / float64(total) * 100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return used, total, percent
}

// networkReachable проверяет доступность сети установкой TCP-соединения.
func networkReachable(addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// netTraffic возвращает суммарный входящий/исходящий трафик по /proc/net/dev.
func netTraffic() (rx, tx uint64) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
	}
	return rx, tx
}

// uptime возвращает аптайм системы в секундах из /proc/uptime.
func uptime() int64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return int64(v)
}
