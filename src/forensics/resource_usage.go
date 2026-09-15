package forensics

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const resourceSampleInterval = 10 * time.Second

type resourceUsage struct {
	PID             int     `json:"pid"`
	CPUPercent      float64 `json:"cpu_percent"`
	RSSMB           uint64  `json:"rss_mb"`
	VirtualMB       uint64  `json:"virtual_mb"`
	ReadMB          uint64  `json:"read_mb"`
	WriteMB         uint64  `json:"write_mb"`
	NetworkRxMB     uint64  `json:"network_rx_mb"`
	NetworkTxMB     uint64  `json:"network_tx_mb"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
	DiskFreeGB      uint64  `json:"disk_free_gb"`
	Timestamp       string  `json:"timestamp"`
	Available       bool    `json:"available"`
	Error           string  `json:"error,omitempty"`
}

type resourceSampler struct {
	mu          sync.Mutex
	previous    resourceCounters
	previousAt  time.Time
	initialized bool
}

type resourceCounters struct {
	processCPUTicks uint64
	readBytes       uint64
	writeBytes      uint64
}

func StartResourceMonitor() {
	sampler := &resourceSampler{}
	sampler.log()
	ticker := time.NewTicker(resourceSampleInterval)
	defer ticker.Stop()
	for range ticker.C {
		sampler.log()
	}
}

func (s *resourceSampler) log() {
	usage := s.sample()
	if Logger == nil {
		return
	}
	Logger.Info("resource_usage",
		zap.Int("pid", usage.PID),
		zap.Float64("cpu_percent", usage.CPUPercent),
		zap.Uint64("rss_mb", usage.RSSMB),
		zap.Uint64("virtual_mb", usage.VirtualMB),
		zap.Uint64("read_mb", usage.ReadMB),
		zap.Uint64("write_mb", usage.WriteMB),
		zap.Uint64("network_rx_mb", usage.NetworkRxMB),
		zap.Uint64("network_tx_mb", usage.NetworkTxMB),
		zap.Float64("disk_used_percent", usage.DiskUsedPercent),
		zap.Uint64("disk_free_gb", usage.DiskFreeGB),
		zap.Bool("available", usage.Available),
		zap.String("error", usage.Error),
	)
}

func (s *resourceSampler) sample() resourceUsage {
	usage := resourceUsage{PID: os.Getpid(), Timestamp: time.Now().Format(time.RFC3339Nano)}
	if runtime.GOOS != "linux" {
		usage.Error = "resource sampling is supported on Linux only"
		return usage
	}

	counters, rss, virtual, err := readProcessResources()
	if err != nil {
		usage.Error = err.Error()
		return usage
	}
	usage.RSSMB = rss / 1024 / 1024
	usage.VirtualMB = virtual / 1024 / 1024
	usage.ReadMB = counters.readBytes / 1024 / 1024
	usage.WriteMB = counters.writeBytes / 1024 / 1024
	usage.NetworkRxMB, usage.NetworkTxMB = readNetworkBytes()
	usage.DiskUsedPercent, usage.DiskFreeGB = readDiskUsage()
	usage.Available = true

	s.mu.Lock()
	if s.initialized {
		elapsed := time.Since(s.previousAt).Seconds()
		if elapsed > 0 {
			usage.CPUPercent = float64(counters.processCPUTicks-s.previous.processCPUTicks) / 100 / elapsed / float64(runtime.NumCPU()) * 100
			if usage.CPUPercent < 0 {
				usage.CPUPercent = 0
			}
		}
	}
	s.previous = counters
	s.previousAt = time.Now()
	s.initialized = true
	s.mu.Unlock()
	return usage
}

func readProcessResources() (resourceCounters, uint64, uint64, error) {
	var counters resourceCounters
	stat, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return counters, 0, 0, err
	}
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+2:]))
	if len(fields) < 22 {
		return counters, 0, 0, syscall.EINVAL
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return counters, 0, 0, err
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return counters, 0, 0, err
	}
	counters.processCPUTicks = utime + stime

	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return counters, 0, 0, err
	}
	var rss, virtual uint64
	for _, line := range strings.Split(string(status), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch fields[0] {
		case "VmRSS:":
			rss = value * 1024
		case "VmSize:":
			virtual = value * 1024
		}
	}

	io, err := os.ReadFile("/proc/self/io")
	if err == nil {
		for _, line := range strings.Split(string(io), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			value, parseErr := strconv.ParseUint(fields[1], 10, 64)
			if parseErr != nil {
				continue
			}
			switch fields[0] {
			case "read_bytes:":
				counters.readBytes = value
			case "write_bytes:":
				counters.writeBytes = value
			}
		}
	}
	return counters, rss, virtual, nil
}

func readNetworkBytes() (uint64, uint64) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()
	var received, transmitted uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 || strings.TrimSpace(parts[0]) == "lo" {
			continue
		}
		rx, rxErr := strconv.ParseUint(fields[0], 10, 64)
		tx, txErr := strconv.ParseUint(fields[8], 10, 64)
		if rxErr == nil && txErr == nil {
			received += rx
			transmitted += tx
		}
	}
	return received / 1024 / 1024, transmitted / 1024 / 1024
}

func readDiskUsage() (float64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err != nil || stat.Blocks == 0 {
		return 0, 0
	}
	free := stat.Bavail * uint64(stat.Bsize)
	total := stat.Blocks * uint64(stat.Bsize)
	return float64(total-free) / float64(total) * 100, free / 1024 / 1024 / 1024
}
