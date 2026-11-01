package provider

import (
	"context"
	"encoding/csv"
	"os/exec"
	"strconv"
	"strings"

	"aurora/internal/model"
)

// GPUs reads NVIDIA telemetry through nvidia-smi. AMD cards are currently
// reported by the kernel but not sampled for power/temperature.
func (l *Live) GPUs(ctx context.Context) ([]model.GPU, error) {
	out, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,uuid,name,driver_version,memory.total,memory.used,utilization.gpu,temperature.gpu,power.draw,power.limit,fan.speed,clocks.gr,clocks.mem",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return []model.GPU{}, nil
	}
	gpus, byUUID := parseNvidiaSMI(string(out))
	appsOut, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-compute-apps=gpu_uuid,pid,process_name,used_memory",
		"--format=csv,noheader,nounits").Output()
	if err == nil {
		for _, process := range parseNvidiaProcesses(string(appsOut)) {
			if idx, ok := byUUID[process.UUID]; ok && idx < len(gpus) {
				gpus[idx].Processes = append(gpus[idx].Processes, process.Process)
			}
		}
	}
	for i := range gpus {
		if gpus[i].Processes == nil {
			gpus[i].Processes = []model.GPUProcess{}
		}
	}
	return gpus, nil
}

func parseNvidiaSMI(text string) ([]model.GPU, map[string]int) {
	var gpus []model.GPU
	byUUID := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := csvFields(line)
		if len(fields) < 13 {
			continue
		}
		gpu := model.GPU{
			Index:          int(number(fields[0])),
			UUID:           fields[1],
			Name:           fields[2],
			Driver:         fields[3],
			MemoryTotal:    uint64(number(fields[4])) * 1024 * 1024,
			MemoryUsed:     uint64(number(fields[5])) * 1024 * 1024,
			Utilization:    number(fields[6]),
			Temperature:    number(fields[7]),
			Power:          number(fields[8]),
			PowerLimit:     number(fields[9]),
			Fan:            number(fields[10]),
			ClockMHz:       int(number(fields[11])),
			MemoryClockMHz: int(number(fields[12])),
			Processes:      []model.GPUProcess{},
		}
		if strings.Contains(strings.ToLower(gpu.Name), "nvidia") || strings.Contains(strings.ToLower(gpu.Name), "rtx") || strings.Contains(strings.ToLower(gpu.Name), "gtx") {
			gpu.Vendor = "NVIDIA"
		}
		byUUID[gpu.UUID] = len(gpus)
		gpus = append(gpus, gpu)
	}
	return gpus, byUUID
}

type nvidiaProcess struct {
	UUID    string
	Process model.GPUProcess
}

func parseNvidiaProcesses(text string) []nvidiaProcess {
	var out []nvidiaProcess
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		fields := csvFields(strings.TrimSpace(line))
		if len(fields) < 4 {
			continue
		}
		out = append(out, nvidiaProcess{
			UUID: fields[0],
			Process: model.GPUProcess{
				PID:    int(number(fields[1])),
				Name:   fields[2],
				Memory: uint64(number(fields[3])) * 1024 * 1024,
			},
		})
	}
	return out
}

func csvFields(line string) []string {
	reader := csv.NewReader(strings.NewReader(line))
	reader.TrimLeadingSpace = true
	fields, err := reader.Read()
	if err != nil {
		return strings.Split(line, ",")
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	return fields
}

func number(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "[N/A]") || strings.EqualFold(value, "N/A") {
		return 0
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return parsed
	}
	// Some nvidia-smi versions append units in the "noheader" output.
	if idx := strings.IndexFunc(value, func(r rune) bool { return (r < '0' || r > '9') && r != '.' && r != '-' }); idx > 0 {
		parsed, _ := strconv.ParseFloat(value[:idx], 64)
		return parsed
	}
	return 0
}
