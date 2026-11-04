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
var zfsCompressionPattern = regexp.MustCompile(`^(off|lz4|zstd|zle|gzip|gzip-[1-9])$`)
var zfsRecordSizePattern = regexp.MustCompile(`^(512|1K|2K|4K|8K|16K|32K|64K|128K|256K|512K|1M)$`)

func (l *Live) ZFSPools(ctx context.Context) ([]model.ZFSPool, error) {
	out, err := exec.CommandContext(ctx, "zpool", "list", "-H", "-p", "-o", "name,size,alloc,free,health").Output()
	if err != nil {
		return nil, nil // ZFS is optional.
	}
	pools := parseZFSPools(string(out))
	datasets, _ := exec.CommandContext(ctx, "zfs", "list", "-H", "-p", "-o", "name,used,avail,refer,mountpoint,type,compression,quota,readonly,recordsize").Output()
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
		dataset := model.ZFSDataset{
			Name: fields[0], Used: used, Available: avail, Refer: refer,
			Mountpoint: fields[4], Type: fields[5],
		}
		if len(fields) >= 10 {
			quota, _ := strconv.ParseUint(fields[7], 10, 64)
			dataset.Compression = fields[6]
			dataset.Quota = quota
			dataset.ReadOnly = fields[8] == "on"
			dataset.RecordSize = fields[9]
		}
		out = append(out, dataset)
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

func (l *Live) CreateZFSDataset(ctx context.Context, opts model.ZFSDatasetCreate) error {
	if err := validateZFSDatasetName(opts.Name); err != nil {
		return err
	}
	compression := defaultString(opts.Compression, "off")
	if !zfsCompressionPattern.MatchString(compression) {
		return errors.New("invalid compression mode")
	}
	recordsize := defaultString(opts.RecordSize, "128K")
	if !zfsRecordSizePattern.MatchString(recordsize) {
		return errors.New("invalid recordsize")
	}
	args := []string{"create", "-o", "compression=" + compression, "-o", "recordsize=" + recordsize}
	if opts.QuotaBytes > 0 {
		args = append(args, "-o", "quota="+strconv.FormatUint(opts.QuotaBytes, 10))
	}
	if opts.ReadOnly {
		args = append(args, "-o", "readonly=on")
	}
	args = append(args, opts.Name)
	if output, err := exec.CommandContext(ctx, "zfs", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("create dataset: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) UpdateZFSDataset(ctx context.Context, opts model.ZFSDatasetUpdate) error {
	if err := validateZFSDatasetName(opts.Name); err != nil {
		return err
	}
	args := []string{"set"}
	if opts.Compression != nil {
		if !zfsCompressionPattern.MatchString(*opts.Compression) {
			return errors.New("invalid compression mode")
		}
		args = append(args, "compression="+*opts.Compression)
	}
	if opts.RecordSize != nil {
		if !zfsRecordSizePattern.MatchString(*opts.RecordSize) {
			return errors.New("invalid recordsize")
		}
		args = append(args, "recordsize="+*opts.RecordSize)
	}
	if opts.QuotaBytes != nil {
		args = append(args, "quota="+strconv.FormatUint(*opts.QuotaBytes, 10))
	}
	if opts.ReadOnly != nil {
		value := "off"
		if *opts.ReadOnly {
			value = "on"
		}
		args = append(args, "readonly="+value)
	}
	if len(args) == 1 {
		return errors.New("no dataset properties supplied")
	}
	args = append(args, opts.Name)
	if output, err := exec.CommandContext(ctx, "zfs", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("update dataset: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) DeleteZFSDataset(ctx context.Context, name string, recursive bool) error {
	if err := validateZFSDatasetName(name); err != nil {
		return err
	}
	args := []string{"destroy"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, name)
	if output, err := exec.CommandContext(ctx, "zfs", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("delete dataset: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) RollbackZFSSnapshot(ctx context.Context, dataset, name string) error {
	if err := validateZFSDatasetName(dataset); err != nil {
		return err
	}
	if !zfsNamePattern.MatchString(name) {
		return errors.New("invalid snapshot name")
	}
	if output, err := exec.CommandContext(ctx, "zfs", "rollback", "-r", dataset+"@"+name).CombinedOutput(); err != nil {
		return fmt.Errorf("rollback snapshot: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) CloneZFSSnapshot(ctx context.Context, opts model.ZFSSnapshotClone) error {
	if err := validateZFSDatasetName(opts.Dataset); err != nil {
		return err
	}
	if !zfsNamePattern.MatchString(opts.Name) {
		return errors.New("invalid snapshot name")
	}
	if err := validateZFSDatasetName(opts.Target); err != nil {
		return err
	}
	if output, err := exec.CommandContext(ctx, "zfs", "clone", opts.Dataset+"@"+opts.Name, opts.Target).CombinedOutput(); err != nil {
		return fmt.Errorf("clone snapshot: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func validateZFSDatasetName(name string) error {
	if !zfsDatasetPattern.MatchString(name) || strings.Contains(name, "..") || strings.Contains(name, "@") || !strings.Contains(name, "/") {
		return errors.New("invalid dataset name")
	}
	return nil
}
