package provider

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"aurora/internal/model"
)

const vmBackupManifestName = "manifest.json"

var (
	qemuProgressPattern = regexp.MustCompile(`\(\s*([0-9]+(?:\.[0-9]+)?)\s*/`)
	qemuPercentPattern  = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)%`)
)

type vmJobRuntime struct {
	cancel context.CancelFunc
	job    model.VMBackup
}

type vmBackupManifest struct {
	Version int                  `json:"version"`
	VMID    string               `json:"vmId"`
	VMName  string               `json:"vmName"`
	Created time.Time            `json:"created"`
	Files   []model.VMBackupFile `json:"files"`
}

type vmDiskPlan struct {
	source string
	name   string
	format string
	size   uint64
}

type vmRestorePolicy struct {
	disks map[string]bool
	nvram string
}

type vmDomainConfig struct {
	Name  string `xml:"name"`
	NVRAM string `xml:"nvram"`
	Disks []struct {
		Type   string `xml:"type,attr"`
		Device string `xml:"device,attr"`
		Source struct {
			File string `xml:"file,attr"`
		} `xml:"source"`
	} `xml:"devices>disk"`
}

func vmDomainRunState(ctx context.Context, name string) (string, error) {
	out, err := exec.CommandContext(ctx, "virsh", "domstate", name).Output()
	if err != nil {
		return "", fmt.Errorf("read VM state: %w", err)
	}
	state := strings.ToLower(strings.TrimSpace(string(out)))
	switch {
	case strings.HasPrefix(state, "running"):
		return "running", nil
	case strings.HasPrefix(state, "paused"):
		return "paused", nil
	case strings.HasPrefix(state, "shut off"), strings.HasPrefix(state, "crashed"):
		return "shut off", nil
	default:
		return state, nil
	}
}

func parseVMDomain(raw []byte) (vmDomainConfig, error) {
	var domain vmDomainConfig
	if err := xml.Unmarshal(raw, &domain); err != nil {
		return domain, fmt.Errorf("parse domain XML: %w", err)
	}
	if strings.TrimSpace(domain.Name) == "" {
		return domain, errors.New("domain XML has no name")
	}
	domain.NVRAM = strings.TrimSpace(domain.NVRAM)
	return domain, nil
}

// vmDiskSources returns file-backed disks in definition order. CD-ROMs,
// network disks and duplicate paths are ignored.
func vmDiskSources(domain vmDomainConfig) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(domain.Disks))
	for _, disk := range domain.Disks {
		path := strings.TrimSpace(disk.Source.File)
		if disk.Type != "file" || disk.Device != "disk" || path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

// vmBackupRoot accepts only an absolute directory under /mnt.
func vmBackupRoot(target string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(target))
	if clean == "" || clean == "." || !filepath.IsAbs(clean) || strings.ContainsRune(clean, 0) {
		return "", errors.New("the backup directory must be an absolute path")
	}
	if clean == "/mnt" || !strings.HasPrefix(clean, "/mnt/") {
		return "", errors.New("the backup directory must be under /mnt")
	}
	return clean, nil
}

func freeBytes(path string) (uint64, error) {
	var info syscall.Statfs_t
	if err := syscall.Statfs(path, &info); err != nil {
		return 0, fmt.Errorf("inspect free space: %w", err)
	}
	return uint64(info.Bavail) * uint64(info.Bsize), nil
}

type qemuInfo struct {
	VirtualSize uint64 `json:"virtual-size"`
	Format      string `json:"format"`
}

func qemuDiskInfo(ctx context.Context, path string) (qemuInfo, error) {
	out, err := exec.CommandContext(ctx, "qemu-img", "info", "--output=json", path).Output()
	if err != nil {
		return qemuInfo{}, fmt.Errorf("inspect disk %s: %w", path, err)
	}
	var info qemuInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return qemuInfo{}, fmt.Errorf("inspect disk %s: %w", path, err)
	}
	if info.VirtualSize == 0 {
		return qemuInfo{}, fmt.Errorf("disk %s has no virtual size", path)
	}
	return info, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeFileSHA256(path string, data []byte) (string, error) {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return fileSHA256(path)
}

func copyFileSHA256(source, destination string) (uint64, string, error) {
	in, err := os.Open(source)
	if err != nil {
		return 0, "", err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, "", err
	}
	defer out.Close()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), in)
	if err != nil {
		return 0, "", err
	}
	return uint64(written), hex.EncodeToString(hash.Sum(nil)), nil
}

func convertDisk(ctx context.Context, source, destination, format string, onProgress func(float64)) error {
	args := []string{"convert", "-p", "-O", format, source, destination}
	cmd := exec.CommandContext(ctx, "qemu-img", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go parseQEMUProgress(stderr, onProgress)
	return cmd.Wait()
}

func parseQEMUProgress(reader io.Reader, onProgress func(float64)) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		for index, value := range data {
			if value == '\r' || value == '\n' {
				if index == 0 {
					// Skip the delimiter instead of emitting an empty token.
					return 1, nil, nil
				}
				return index + 1, data[:index], nil
			}
		}
		if atEOF {
			if len(data) == 0 {
				return 0, nil, nil
			}
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	for scanner.Scan() {
		line := scanner.Text()
		matches := qemuProgressPattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			matches = qemuPercentPattern.FindStringSubmatch(line)
		}
		if len(matches) != 2 {
			continue
		}
		var percent float64
		if _, err := fmt.Sscanf(matches[1], "%f", &percent); err != nil {
			continue
		}
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		onProgress(percent)
	}
}

func (l *Live) jobSnapshot(id string) (model.VMBackup, bool) {
	l.vmJobMu.Lock()
	defer l.vmJobMu.Unlock()
	runtime, ok := l.vmJobs[id]
	if !ok || runtime == nil {
		return model.VMBackup{}, false
	}
	job := runtime.job
	job.Files = append([]model.VMBackupFile(nil), runtime.job.Files...)
	return job, true
}

func (l *Live) updateJob(id string, mutate func(*model.VMBackup)) {
	l.vmJobMu.Lock()
	defer l.vmJobMu.Unlock()
	if runtime := l.vmJobs[id]; runtime != nil {
		mutate(&runtime.job)
	}
}

func (l *Live) finishJob(id, state, phase, message string, cause error) {
	now := time.Now()
	l.vmJobMu.Lock()
	defer l.vmJobMu.Unlock()
	runtime := l.vmJobs[id]
	if runtime == nil {
		return
	}
	runtime.job.State = state
	runtime.job.Phase = phase
	runtime.job.Message = message
	runtime.job.FinishedAt = &now
	if cause != nil {
		runtime.job.Error = cause.Error()
	}
	runtime.cancel = nil
}

func (l *Live) putJob(id string, job model.VMBackup, cancel context.CancelFunc) {
	l.vmJobMu.Lock()
	if l.vmJobs == nil {
		l.vmJobs = make(map[string]*vmJobRuntime)
	}
	if previous := l.vmJobs[id]; previous != nil && previous.cancel != nil {
		previous.cancel()
	}
	l.vmJobs[id] = &vmJobRuntime{cancel: cancel, job: job}
	l.vmJobMu.Unlock()
}

// StartVMBackup snapshots a stopped domain (XML, NVRAM and every file-backed
// disk) into a directory under /mnt. It returns immediately with a job that can
// be polled through VMBackupStatus.
func (l *Live) StartVMBackup(ctx context.Context, id, targetDir string) (model.VMBackup, error) {
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return model.VMBackup{}, err
	}
	state, err := vmDomainRunState(ctx, name)
	if err != nil {
		return model.VMBackup{}, err
	}
	if state != "shut off" {
		return model.VMBackup{}, errors.New("shut down the virtual machine before backing it up")
	}
	root, err := vmBackupRoot(targetDir)
	if err != nil {
		return model.VMBackup{}, err
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return model.VMBackup{}, fmt.Errorf("create backup directory: %w", err)
	}
	dir := filepath.Join(root, fmt.Sprintf("aurora-%s-%s", slug(name), time.Now().Format("20060102-150405")))
	if err := os.Mkdir(dir, 0o750); err != nil {
		return model.VMBackup{}, fmt.Errorf("create backup folder: %w", err)
	}
	job := model.VMBackup{
		ID: newHistoryID(), Kind: "backup", VMID: id, VMName: name, Dir: dir,
		State: "running", Phase: "preparing", Message: "Reading the domain definition",
		StartedAt: time.Now(), Files: []model.VMBackupFile{},
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	l.putJob(id, job, cancel)
	go l.runVMBackup(jobCtx, id, name, dir)
	return job, nil
}

func (l *Live) VMBackupStatus(_ context.Context, id string) (model.VMBackup, error) {
	if job, ok := l.jobSnapshot(id); ok {
		return job, nil
	}
	return model.VMBackup{State: "idle", Phase: "idle", Files: []model.VMBackupFile{}}, nil
}

func (l *Live) CancelVMBackup(_ context.Context, id string) error {
	l.vmJobMu.Lock()
	defer l.vmJobMu.Unlock()
	runtime := l.vmJobs[id]
	if runtime == nil || runtime.cancel == nil {
		return errors.New("no backup or restore is running")
	}
	if runtime.job.State != "running" {
		return errors.New("the operation is not running")
	}
	runtime.job.State = "cancelling"
	runtime.job.Message = "Cancelling"
	runtime.cancel()
	return nil
}

func (l *Live) runVMBackup(ctx context.Context, id, name, dir string) {
	fail := func(err error) {
		_ = os.RemoveAll(dir)
		if errors.Is(err, context.Canceled) {
			l.finishJob(id, "cancelled", "cancelled", "Backup cancelled", nil)
			return
		}
		l.finishJob(id, "failed", "failed", "Backup failed", err)
	}
	l.updateJob(id, func(job *model.VMBackup) {
		job.Phase = "xml"
		job.Message = "Saving the domain definition"
	})
	raw, err := exec.CommandContext(ctx, "virsh", "dumpxml", name).Output()
	if err != nil {
		fail(fmt.Errorf("read domain XML: %w", err))
		return
	}
	domain, err := parseVMDomain(raw)
	if err != nil {
		fail(err)
		return
	}
	disks := vmDiskSources(domain)
	plans := make([]vmDiskPlan, 0, len(disks))
	var total uint64 = uint64(len(raw))
	for index, source := range disks {
		info, err := qemuDiskInfo(ctx, source)
		if err != nil {
			fail(err)
			return
		}
		plans = append(plans, vmDiskPlan{
			source: source,
			name:   fmt.Sprintf("disk-%02d.qcow2", index+1),
			format: info.Format,
			size:   info.VirtualSize,
		})
		total += info.VirtualSize
	}
	var nvramSize uint64
	if domain.NVRAM != "" {
		if info, err := os.Stat(domain.NVRAM); err == nil {
			nvramSize = uint64(info.Size())
			total += nvramSize
		}
	}
	l.updateJob(id, func(job *model.VMBackup) {
		job.TotalBytes = total
	})
	if free, err := freeBytes(dir); err != nil {
		fail(err)
		return
	} else if free < total+total/10 {
		fail(fmt.Errorf("not enough free space in %s: %d bytes available, %d recommended", dir, free, total+total/10))
		return
	}
	var done uint64
	files := make([]model.VMBackupFile, 0, len(plans)+2)

	xmlName := "domain.xml"
	sum, err := writeFileSHA256(filepath.Join(dir, xmlName), raw)
	if err != nil {
		fail(err)
		return
	}
	files = append(files, model.VMBackupFile{Name: xmlName, Kind: "domain", Size: uint64(len(raw)), SHA256: sum})
	done += uint64(len(raw))
	l.updateJob(id, func(job *model.VMBackup) { job.DoneBytes = done })

	if domain.NVRAM != "" {
		l.updateJob(id, func(job *model.VMBackup) {
			job.Phase = "nvram"
			job.Message = "Copying NVRAM"
		})
		size, sum, err := copyFileSHA256(domain.NVRAM, filepath.Join(dir, "nvram.fd"))
		if err != nil {
			fail(fmt.Errorf("copy NVRAM: %w", err))
			return
		}
		files = append(files, model.VMBackupFile{
			Name: "nvram.fd", Kind: "nvram", Target: domain.NVRAM, Size: size, SHA256: sum,
		})
		done += size
		l.updateJob(id, func(job *model.VMBackup) { job.DoneBytes = done })
	}

	for index, plan := range plans {
		l.updateJob(id, func(job *model.VMBackup) {
			job.Phase = "disk"
			job.Message = fmt.Sprintf("Copying disk %d/%d", index+1, len(plans))
		})
		base := done
		target := filepath.Join(dir, plan.name)
		err := convertDisk(ctx, plan.source, target, "qcow2", func(percent float64) {
			l.updateJob(id, func(job *model.VMBackup) {
				job.DoneBytes = base + uint64(percent/100*float64(plan.size))
			})
		})
		if err != nil {
			fail(fmt.Errorf("copy disk %s: %w", plan.source, err))
			return
		}
		info, err := os.Stat(target)
		if err != nil {
			fail(err)
			return
		}
		sum, err := fileSHA256(target)
		if err != nil {
			fail(err)
			return
		}
		files = append(files, model.VMBackupFile{
			Name: plan.name, Kind: "disk", Target: plan.source, Format: plan.format,
			Size: uint64(info.Size()), SHA256: sum,
		})
		done = base + plan.size
		l.updateJob(id, func(job *model.VMBackup) {
			job.DoneBytes = done
			job.Files = append([]model.VMBackupFile(nil), files...)
		})
	}

	l.updateJob(id, func(job *model.VMBackup) {
		job.Phase = "manifest"
		job.Message = "Writing the manifest"
	})
	manifest := vmBackupManifest{
		Version: 1, VMID: id, VMName: name, Created: time.Now().UTC(), Files: files,
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fail(err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, vmBackupManifestName), encoded, 0o600); err != nil {
		fail(err)
		return
	}
	l.updateJob(id, func(job *model.VMBackup) {
		job.DoneBytes = total
		job.Files = append([]model.VMBackupFile(nil), files...)
	})
	l.recordConfigEvent("vm.backup", fmt.Sprintf("Backed up %s to %s", name, dir))
	l.finishJob(id, "completed", "done", "Backup complete", nil)
}

func readVMBackupManifest(dir string) (vmBackupManifest, error) {
	var manifest vmBackupManifest
	raw, err := os.ReadFile(filepath.Join(dir, vmBackupManifestName))
	if err != nil {
		return manifest, fmt.Errorf("read backup manifest: %w", err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("parse backup manifest: %w", err)
	}
	if manifest.Version != 1 || manifest.VMName == "" || len(manifest.Files) == 0 {
		return manifest, errors.New("unsupported or incomplete backup manifest")
	}
	return manifest, nil
}

// StartVMRestore restores a stopped domain from an Aurora backup directory.
// Existing disks are moved aside before being replaced and restored on error.
func (l *Live) StartVMRestore(ctx context.Context, id, backupDir string, confirm bool) (model.VMBackup, error) {
	if !confirm {
		return model.VMBackup{}, errors.New("restoring a backup requires explicit confirmation")
	}
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return model.VMBackup{}, err
	}
	state, err := vmDomainRunState(ctx, name)
	if err != nil {
		return model.VMBackup{}, err
	}
	if state != "shut off" {
		return model.VMBackup{}, errors.New("shut down the virtual machine before restoring it")
	}
	dir, err := vmBackupRoot(backupDir)
	if err != nil {
		return model.VMBackup{}, err
	}
	manifest, err := readVMBackupManifest(dir)
	if err != nil {
		return model.VMBackup{}, err
	}
	if manifest.VMName != name {
		return model.VMBackup{}, fmt.Errorf("this backup belongs to %s, not %s", manifest.VMName, name)
	}
	// Never trust restore targets from the manifest: they must match the
	// current domain definition.
	current, err := exec.CommandContext(ctx, "virsh", "dumpxml", name).Output()
	if err != nil {
		return model.VMBackup{}, fmt.Errorf("read the current domain: %w", err)
	}
	currentDomain, err := parseVMDomain(current)
	if err != nil {
		return model.VMBackup{}, err
	}
	policy := vmRestorePolicy{disks: make(map[string]bool), nvram: currentDomain.NVRAM}
	for _, disk := range vmDiskSources(currentDomain) {
		policy.disks[disk] = true
	}
	job := model.VMBackup{
		ID: newHistoryID(), Kind: "restore", VMID: id, VMName: name, Dir: dir,
		State: "running", Phase: "verifying", Message: "Verifying the backup",
		StartedAt: time.Now(), Files: manifest.Files,
	}
	for _, file := range manifest.Files {
		job.TotalBytes += file.Size
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	l.putJob(id, job, cancel)
	go l.runVMRestore(jobCtx, id, name, dir, manifest, policy)
	return job, nil
}

func (l *Live) runVMRestore(ctx context.Context, id, name, dir string, manifest vmBackupManifest, policy vmRestorePolicy) {
	var replaced []struct{ target, backup string }
	rollback := func() {
		for index := len(replaced) - 1; index >= 0; index-- {
			entry := replaced[index]
			_ = os.Remove(entry.target)
			if entry.backup != "" {
				_ = os.Rename(entry.backup, entry.target)
			}
		}
	}
	fail := func(err error) {
		rollback()
		if errors.Is(err, context.Canceled) {
			l.finishJob(id, "cancelled", "cancelled", "Restore cancelled", nil)
			return
		}
		l.finishJob(id, "failed", "failed", "Restore failed", err)
	}

	var done uint64
	for index, file := range manifest.Files {
		path := filepath.Join(dir, file.Name)
		info, err := os.Stat(path)
		if err != nil || uint64(info.Size()) != file.Size {
			fail(fmt.Errorf("backup file %s is missing or truncated", file.Name))
			return
		}
		sum, err := fileSHA256(path)
		if err != nil || sum != file.SHA256 {
			fail(fmt.Errorf("checksum mismatch for %s", file.Name))
			return
		}
		done += file.Size
		l.updateJob(id, func(job *model.VMBackup) {
			job.DoneBytes = done
			job.Message = fmt.Sprintf("Verified %d/%d files", index+1, len(manifest.Files))
		})
	}

	stamp := time.Now().Format("20060102-150405")
	backupOf := func(target string) string {
		return fmt.Sprintf("%s.aurora-pre-restore-%s", target, stamp)
	}

	var diskTotal uint64
	for _, file := range manifest.Files {
		if file.Kind == "disk" {
			diskTotal += file.Size
		}
	}
	if len(manifest.Files) > 0 && diskTotal > 0 {
		firstDisk := ""
		for _, file := range manifest.Files {
			if file.Kind == "disk" && file.Target != "" {
				firstDisk = filepath.Dir(file.Target)
				break
			}
		}
		if firstDisk != "" {
			free, err := freeBytes(firstDisk)
			if err != nil {
				fail(err)
				return
			}
			if free < diskTotal+diskTotal/20 {
				fail(fmt.Errorf("not enough free space in %s: %d bytes available, %d required", firstDisk, free, diskTotal+diskTotal/20))
				return
			}
		}
	}

	for _, file := range manifest.Files {
		if file.Kind != "disk" {
			continue
		}
		if file.Target == "" || file.Format == "" {
			fail(fmt.Errorf("backup file %s has no restore target", file.Name))
			return
		}
		if !policy.disks[file.Target] {
			fail(fmt.Errorf("backup file %s targets a disk that is not part of this virtual machine", file.Name))
			return
		}
		if _, err := vmBackupRoot(filepath.Dir(file.Target)); err != nil {
			fail(fmt.Errorf("invalid restore target %s: %w", file.Target, err))
			return
		}
		l.updateJob(id, func(job *model.VMBackup) {
			job.Phase = "disk"
			job.Message = "Restoring " + filepath.Base(file.Target)
		})
		tmp := fmt.Sprintf("%s.aurora-restore-%s", file.Target, stamp)
		if err := convertDisk(ctx, filepath.Join(dir, file.Name), tmp, file.Format, nil); err != nil {
			_ = os.Remove(tmp)
			fail(fmt.Errorf("restore disk %s: %w", file.Target, err))
			return
		}
		backup := ""
		if _, err := os.Stat(file.Target); err == nil {
			backup = backupOf(file.Target)
			if err := os.Rename(file.Target, backup); err != nil {
				_ = os.Remove(tmp)
				fail(fmt.Errorf("move existing disk %s: %w", file.Target, err))
				return
			}
		}
		if err := os.Rename(tmp, file.Target); err != nil {
			_ = os.Remove(tmp)
			fail(fmt.Errorf("install restored disk %s: %w", file.Target, err))
			return
		}
		replaced = append(replaced, struct{ target, backup string }{target: file.Target, backup: backup})
	}

	for _, file := range manifest.Files {
		if file.Kind != "nvram" {
			continue
		}
		if file.Target == "" || policy.nvram == "" || file.Target != policy.nvram {
			fail(errors.New("backup NVRAM does not match the current domain"))
			return
		}
		l.updateJob(id, func(job *model.VMBackup) {
			job.Phase = "nvram"
			job.Message = "Restoring NVRAM"
		})
		tmp := file.Target + ".aurora-restore-" + stamp
		if _, _, err := copyFileSHA256(filepath.Join(dir, file.Name), tmp); err != nil {
			_ = os.Remove(tmp)
			fail(fmt.Errorf("restore NVRAM: %w", err))
			return
		}
		backup := ""
		if _, err := os.Stat(file.Target); err == nil {
			backup = backupOf(file.Target)
			if err := os.Rename(file.Target, backup); err != nil {
				_ = os.Remove(tmp)
				fail(fmt.Errorf("move existing NVRAM: %w", err))
				return
			}
		}
		if err := os.Rename(tmp, file.Target); err != nil {
			_ = os.Remove(tmp)
			fail(fmt.Errorf("install restored NVRAM: %w", err))
			return
		}
		replaced = append(replaced, struct{ target, backup string }{target: file.Target, backup: backup})
	}

	l.updateJob(id, func(job *model.VMBackup) {
		job.Phase = "domain"
		job.Message = "Redefining the domain"
	})
	xmlPath := filepath.Join(dir, "domain.xml")
	raw, err := os.ReadFile(xmlPath)
	if err != nil {
		fail(err)
		return
	}
	tmpXML, err := os.CreateTemp("", "aurora-vm-restore-*.xml")
	if err != nil {
		fail(err)
		return
	}
	tmpName := tmpXML.Name()
	_ = tmpXML.Close()
	defer os.Remove(tmpName)
	if err := os.WriteFile(tmpName, raw, 0o600); err != nil {
		fail(err)
		return
	}
	restoredDomain, err := parseVMDomain(raw)
	if err != nil || restoredDomain.Name != name {
		fail(errors.New("the backup domain definition does not match this virtual machine"))
		return
	}
	if output, err := exec.CommandContext(ctx, "virsh", "define", tmpName).CombinedOutput(); err != nil {
		fail(fmt.Errorf("define restored domain: %w (%s)", err, strings.TrimSpace(string(output))))
		return
	}

	l.recordConfigEvent("vm.restore", fmt.Sprintf("Restored %s from %s", name, dir))
	l.updateJob(id, func(job *model.VMBackup) { job.DoneBytes = job.TotalBytes })
	l.finishJob(id, "completed", "done", "Restore complete", nil)
}
