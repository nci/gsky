package gdalprocess

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type procInfo struct {
	KBytes  map[string]int64
	Strings map[string]string
}

func parseProcInfo(procPath string, lookupKeys []string) (*procInfo, error) {
	data, err := ioutil.ReadFile(procPath)
	if err != nil {
		return nil, err
	}

	keysKV := make(map[string]bool)
	for _, key := range lookupKeys {
		keysKV[key] = false
	}

	infoLookup := &procInfo{KBytes: make(map[string]int64), Strings: make(map[string]string)}

	numFound := 0
	allFound := false
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 2 {
			continue
		}

		key := strings.TrimSpace(fields[0])
		if _, found := keysKV[key]; !found {
			continue
		} else {
			keysKV[key] = true
			numFound++
		}

		val := strings.TrimSpace(fields[1])
		unitSuffix := len(val) - 2
		if val[unitSuffix:] != "kB" {
			infoLookup.Strings[key] = val
		} else {
			valInt, err := strconv.ParseInt(val[:unitSuffix-1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s: failed to parse %s", procPath, line)
			}

			infoLookup.KBytes[key] = valInt
		}

		if numFound == len(keysKV) {
			allFound = true
			break
		}
	}

	if !allFound {
		for k, v := range keysKV {
			if !v {
				return nil, fmt.Errorf("%s: %s not found", procPath, k)
			}
		}
	}

	return infoLookup, nil
}

type memoryInfo struct {
	TotalMemory     int64
	AvailableMemory int64
}

func getMemoryInfo() (*memoryInfo, error) {
	info, err := parseProcInfo("/proc/meminfo", []string{"MemTotal", "MemAvailable"})
	if err != nil {
		return nil, err
	}
	return &memoryInfo{TotalMemory: info.KBytes["MemTotal"], AvailableMemory: info.KBytes["MemAvailable"]}, nil
}

type processStatus struct {
	Name  string
	VmRSS int64
	Pid   int
}

func findProcessStatus(pattern *regexp.Regexp) ([]*processStatus, error) {
	var procStatus []*processStatus
	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		return procStatus, nil
	}

	currentPid := os.Getpid()
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		dirName := file.Name()
		pid64, err := strconv.ParseInt(dirName, 10, 64)
		if err != nil {
			continue
		}

		pid := int(pid64)
		if pid <= 1 || pid == currentPid {
			continue
		}

		statusPath := fmt.Sprintf("/proc/%s/status", dirName)
		info, err := parseProcInfo(statusPath, []string{"Name", "VmRSS"})
		if err != nil {
			continue
		}

		if !pattern.MatchString(info.Strings["Name"]) {
			continue
		}

		statusInfo := &processStatus{Name: info.Strings["Name"], Pid: pid, VmRSS: info.KBytes["VmRSS"]}
		procStatus = append(procStatus, statusInfo)
	}

	return procStatus, nil
}

type OOMMonitor struct {
	ExecMatch    string
	OOMThreshold int
	Verbose      bool
}

func NewOOMMonitor(execMatch string, oomThreshold int, verbose bool) *OOMMonitor {
	return &OOMMonitor{
		ExecMatch:    execMatch,
		OOMThreshold: oomThreshold,
		Verbose:      verbose,
	}
}

func (mon *OOMMonitor) getPollInterval(memInfo *memoryInfo) int {
	// expected memory fill rate: 6000 MB/s
	fillRate := 6000 * 1024

	remaining := int(memInfo.AvailableMemory) - mon.OOMThreshold
	if remaining <= 0 {
		return 0
	}

	// predicted time to fill in ms
	predictedTime := int(float32(remaining) / float32(fillRate) * 1000.0)
	if predictedTime < 100 {
		return 100
	}

	if predictedTime > 1000 {
		return 1000
	}

	return predictedTime
}

func (mon *OOMMonitor) StartMonitorLoop() error {
	pattern := regexp.MustCompile(mon.ExecMatch)

	isMemInfoFirst := true
	isNoProcessFound := true
	for {
		memInfo, err := getMemoryInfo()
		if err != nil {
			return err
		}

		if mon.Verbose && isMemInfoFirst {
			log.Printf("meminfo (KB), total: %d, available: %d, OOM threshold: %d", memInfo.TotalMemory, memInfo.AvailableMemory, mon.OOMThreshold)
			isMemInfoFirst = false
		}

		interval := mon.getPollInterval(memInfo)
		if interval >= 100 {
			time.Sleep(time.Duration(interval) * time.Millisecond)
			continue
		}

		procStatus, err := findProcessStatus(pattern)
		if err != nil {
			return err
		}

		maxProc := &processStatus{Pid: -1, VmRSS: -1}
		for _, proc := range procStatus {
			if proc.VmRSS > maxProc.VmRSS {
				maxProc = proc
			}
		}

		if maxProc.Pid > 0 {
			syscall.Kill(maxProc.Pid, syscall.SIGKILL)
			if mon.Verbose {
				log.Printf("OOM SIGKILL sent to process: %s, PID: %d", maxProc.Name, maxProc.Pid)
			}

			for i := 1; i < 100; i++ {
				err := syscall.Kill(maxProc.Pid, 0)
				if err != nil {
					if mon.Verbose {
						log.Printf("OOM terminated process in %.1f secs: %s, PID: %d", float32(i)*0.1, maxProc.Name, maxProc.Pid)
					}
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		} else {
			if mon.Verbose && isNoProcessFound {
				log.Printf("no process found with exec matching pattern: %s", mon.ExecMatch)
				isNoProcessFound = false
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}
}
