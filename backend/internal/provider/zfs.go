package provider

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"aurora/internal/model"
)

var zfsPoolPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
var zfsDatasetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,255}$`)
var zfsNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

func (l *Live) ZFSPools(ctx context.Context) ([]model.ZFSPool, error) {
	out, err := exec.CommandContext(ctx, "zpool", "list", "-H", "-p", "-o", "name,size,alloc,free,health").Output()
	if err != nil {
		return nil, nil // ZFS is optional.
	}
	pools := parseZFSPools(string(out))
	datasets, _ := exec.CommandContext(ctx, "zfs", "list", "-H", "-p", "-o", "name,used,avail,refer,mountpoint,type").Output()
	for _, dataset := range parseZFSDatasets(string(datasets)) {
		poolName := strings.SplitN(dataset.Name, "/", 2)[0]
		for i := range pools {
			if pools[i].Name == poolName {
				pools[i].Datasets = append(pools[i].Datasets, dataset)
			}
		}
	}
	return pools, nil
}

func parseZFSPools(output string) []model.ZFSPool {
	var out []model.ZFSPool
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		size, _ := strconv.ParseUint(fields[1], 10, 64)
		alloc, _ := strconv.ParseUint(fields[2], 10, 64)
		free, _ := strconv.ParseUint(fields[3], 10, 64)
		out = append(out, model.ZFSPool{Name: fields[0], Size: size, Allocated: alloc, Free: free, Health: fields[4]})
	}
	return out
}

func parseZFSDatasets(output string) []model.ZFSDataset {
	var out []model.ZFSDataset
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			fields = strings.Fields(line)
		}
		if len(fields) < 6 {
			continue
		}
		used, _ := strconv.ParseUint(fields[1], 10, 64)
		avail, _ := strconv.ParseUint(fields[2], 10, 64)
		refer, _ := strconv.ParseUint(fields[3], 10, 64)
		out = append(out, model.ZFSDataset{
			Name: fields[0], Used: used, Available: avail, Refer: refer,
			Mountpoint: fields[4], Type: fields[5],
		})
	}
	return out
}

func (l *Live) ScrubZFSPool(ctx context.Context, name string) error {
	if !zfsPoolPattern.MatchString(name) {
		return errors.New("invalid pool name")
	}
	if output, err := exec.CommandContext(ctx, "zpool", "scrub", name).CombinedOutput(); err != nil {
		return fmt.Errorf("start scrub: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) ZFSSnapshots(ctx context.Context, dataset string) ([]model.ZFSSnapshot, error) {
	if !zfsDatasetPattern.MatchString(dataset) || strings.Contains(dataset, "..") {
		return nil, errors.New("invalid dataset name")
	}
	output, err := exec.CommandContext(ctx, "zfs", "list", "-H", "-p", "-t", "snapshot", "-d", "1", "-o", "name,used,creation", dataset).Output()
	if err != nil {
		return nil, err
	}
	var out []model.ZFSSnapshot
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		used, _ := strconv.ParseUint(fields[1], 10, 64)
		created, _ := strconv.ParseInt(fields[2], 10, 64)
		name := fields[0]
		snapshot := name
		if index := strings.Index(name, "@"); index >= 0 {
			snapshot = name[index+1:]
		}
		out = append(out, model.ZFSSnapshot{Name: snapshot, Dataset: dataset, Used: used, Created: created})
	}
	return out, nil
}

func (l *Live) CreateZFSSnapshot(ctx context.Context, dataset, name string) error {
	if !zfsDatasetPattern.MatchString(dataset) || strings.Contains(dataset, "..") || !zfsNamePattern.MatchString(name) {
		return errors.New("invalid snapshot name")
	}
	if output, err := exec.CommandContext(ctx, "zfs", "snapshot", dataset+"@"+name).CombinedOutput(); err != nil {
		return fmt.Errorf("create snapshot: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) DeleteZFSSnapshot(ctx context.Context, dataset, name string) error {
	if !zfsDatasetPattern.MatchString(dataset) || strings.Contains(dataset, "..") || !zfsNamePattern.MatchString(name) {
		return errors.New("invalid snapshot name")
	}
	if output, err := exec.CommandContext(ctx, "zfs", "destroy", dataset+"@"+name).CombinedOutput(); err != nil {
		return fmt.Errorf("delete snapshot: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}
