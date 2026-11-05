package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"aurora/internal/model"
)

type balanceRuntime struct {
	cancel      context.CancelFunc
	job         model.BalanceJob
	destFree    map[string]uint64
	hardlinks   map[string]string
	startedCopy time.Time
	log         []string
}

var balanceExcludedMounts = map[string]bool{
	"user": true, "disks": true, "remotes": true, "addons": true,
	"rootshare": true, "zfs": true,
}

// BalanceDisks lists mounted array disks and pools that can take part in a
// rebalance operation.
func (l *Live) mountRoot() string {
	if l.mountBase != "" {
		return l.mountBase
	}
	return "/mnt"
}

func (l *Live) BalanceDisks(_ context.Context) ([]model.BalanceDisk, error) {
	entries, err := os.ReadDir(l.mountRoot())
	if err != nil {
		return []model.BalanceDisk{}, nil
	}
	disks := make([]model.BalanceDisk, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := entry.Name()
		if balanceExcludedMounts[name] {
			continue
		}
		if strings.HasPrefix(name, "disk") && !isDigits(strings.TrimPrefix(name, "disk")) {
			continue
		}
		mount := filepath.Join(l.mountRoot(), name)
		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}
		size := uint64(stat.Blocks) * uint64(stat.Bsize)
		free := uint64(stat.Bavail) * uint64(stat.Bsize)
		role := "pool"
		if strings.HasPrefix(name, "disk") {
			role = "data"
		}
		if strings.EqualFold(name, "cache") {
			role = "cache"
		}
		disks = append(disks, model.BalanceDisk{
			Name: name, Mount: mount, Role: role,
			Size: size, Free: free, Used: size - minUint64(size, free),
		})
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Name < disks[j].Name })
	return disks, nil
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// BalanceBrowse lists a directory on a disk for the selective move picker.
func (l *Live) BalanceBrowse(ctx context.Context, disk, relative string) ([]model.BalanceEntry, error) {
	disks, err := l.BalanceDisks(ctx)
	if err != nil {
		return nil, err
	}
	source, ok := balanceDiskByName(disks, disk)
	if !ok {
		return nil, errors.New("unknown disk")
	}
	cleaned, ok := cleanRelative(relative)
	if !ok {
		return nil, errors.New("invalid path")
	}
	directory := filepath.Join(source.Mount, filepath.FromSlash(cleaned))
	if !withinMount(source.Mount, directory) {
		return nil, errors.New("path escapes the disk")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	result := make([]model.BalanceEntry, 0, len(entries))
	for _, entry := range entries {
		if len(result) >= 500 {
			break
		}
		info, err := os.Lstat(filepath.Join(directory, entry.Name()))
		if err != nil {
			continue
		}
		relativePath := path.Join(cleaned, entry.Name())
		size := safeUint64(info.Size())
		if info.IsDir() {
			size = quickDirSize(ctx, filepath.Join(directory, entry.Name()))
		}
		result = append(result, model.BalanceEntry{
			Name: entry.Name(), Path: relativePath, Dir: info.IsDir(),
			Size: size, Modified: info.ModTime(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Dir != result[j].Dir {
			return result[i].Dir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func quickDirSize(ctx context.Context, directory string) uint64 {
	commandCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(commandCtx, "du", "-sb", directory).Output()
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return 0
	}
	size, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0
	}
	return size
}

func cleanRelative(relative string) (string, bool) {
	cleaned := path.Clean("/" + strings.ReplaceAll(relative, "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

func withinMount(mount, target string) bool {
	mount = filepath.Clean(mount)
	target = filepath.Clean(target)
	return target == mount || strings.HasPrefix(target, mount+string(os.PathSeparator))
}

func balanceDiskByName(disks []model.BalanceDisk, name string) (model.BalanceDisk, bool) {
	for _, disk := range disks {
		if disk.Name == name {
			return disk, true
		}
	}
	return model.BalanceDisk{}, false
}

type balanceFile struct {
	Rel     string
	Size    uint64
	Mode    os.FileMode
	ModTime time.Time
	Link    string
	Inode   uint64
	Device  uint64
}

func (l *Live) StartBalance(ctx context.Context, opts model.BalanceOptions) (model.BalanceJob, error) {
	l.balanceMu.Lock()
	running := l.balance != nil && (l.balance.job.State == "scanning" || l.balance.job.State == "running" || l.balance.job.State == "cancelling")
	l.balanceMu.Unlock()
	if running {
		return model.BalanceJob{}, errors.New("a balance job is already running")
	}

	disks, err := l.BalanceDisks(ctx)
	if err != nil {
		return model.BalanceJob{}, err
	}
	source, ok := balanceDiskByName(disks, opts.Source)
	if !ok {
		return model.BalanceJob{}, errors.New("unknown source disk")
	}
	destinations := make([]string, 0, len(opts.Destinations))
	for _, name := range opts.Destinations {
		if name == opts.Source {
			continue
		}
		if _, ok := balanceDiskByName(disks, name); ok {
			destinations = append(destinations, name)
		}
	}
	if len(destinations) == 0 {
		for _, disk := range disks {
			if disk.Name != opts.Source {
				destinations = append(destinations, disk.Name)
			}
		}
	}
	if len(destinations) == 0 {
		return model.BalanceJob{}, errors.New("no destination disk available")
	}

	selected := make([]string, 0)
	for _, relative := range opts.Paths {
		cleaned, ok := cleanRelative(relative)
		if !ok || cleaned == "" {
			return model.BalanceJob{}, fmt.Errorf("invalid path %q", relative)
		}
		selected = append(selected, cleaned)
	}
	if len(selected) == 0 {
		entries, err := os.ReadDir(source.Mount)
		if err != nil {
			return model.BalanceJob{}, err
		}
		for _, entry := range entries {
			if entry.Name() == "lost+found" || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			selected = append(selected, entry.Name())
		}
	}
	if len(selected) == 0 {
		return model.BalanceJob{}, errors.New("the source disk is empty")
	}

	mode := "selection"
	if len(opts.Paths) == 0 {
		mode = "empty"
	}
	runCtx, cancel := context.WithCancel(context.Background())
	job := model.BalanceJob{
		ID: time.Now().UTC().Format("20060102-150405"), Source: opts.Source,
		Destinations: destinations, Mode: mode, State: "scanning", Phase: "scanning",
		Started: time.Now().UTC(), DryRun: opts.DryRun, DeleteSource: opts.DeleteSource,
		Files: []model.BalanceMove{}, Log: []string{}, Errors: []string{},
	}
	runtime := &balanceRuntime{
		cancel: cancel, job: job, destFree: map[string]uint64{},
		hardlinks: map[string]string{}, log: []string{},
	}
	for _, name := range destinations {
		if disk, ok := balanceDiskByName(disks, name); ok {
			runtime.destFree[name] = disk.Free
		}
	}
	l.balanceMu.Lock()
	l.balance = runtime
	l.balanceMu.Unlock()

	go l.runBalance(runCtx, runtime, source, disks, selected, opts)
	return l.balanceSnapshot(runtime), nil
}

func (l *Live) BalanceStatus(_ context.Context) (model.BalanceJob, error) {
	l.balanceMu.Lock()
	defer l.balanceMu.Unlock()
	if l.balance != nil {
		return snapshotBalance(l.balance.job), nil
	}
	if len(l.balanceHistory) > 0 {
		return snapshotBalance(l.balanceHistory[0]), nil
	}
	return model.BalanceJob{State: "idle", Phase: "idle", Files: []model.BalanceMove{}, Log: []string{}, Errors: []string{}}, nil
}

func (l *Live) BalanceHistory(_ context.Context) ([]model.BalanceJob, error) {
	l.balanceMu.Lock()
	defer l.balanceMu.Unlock()
	history := make([]model.BalanceJob, len(l.balanceHistory))
	for index, job := range l.balanceHistory {
		history[index] = snapshotBalance(job)
	}
	return history, nil
}

func (l *Live) CancelBalance(_ context.Context) error {
	l.balanceMu.Lock()
	defer l.balanceMu.Unlock()
	if l.balance == nil {
		return errors.New("no balance job is running")
	}
	if l.balance.job.State != "scanning" && l.balance.job.State != "running" {
		return errors.New("the balance job is not running")
	}
	l.balance.job.State = "cancelling"
	l.balance.cancel()
	return nil
}

func snapshotBalance(job model.BalanceJob) model.BalanceJob {
	copied := job
	copied.Files = append([]model.BalanceMove(nil), job.Files...)
	copied.Log = append([]string(nil), job.Log...)
	copied.Errors = append([]string(nil), job.Errors...)
	copied.Destinations = append([]string(nil), job.Destinations...)
	return copied
}

func (l *Live) balanceSnapshot(runtime *balanceRuntime) model.BalanceJob {
	l.balanceMu.Lock()
	defer l.balanceMu.Unlock()
	return snapshotBalance(runtime.job)
}

func (l *Live) runBalance(ctx context.Context, runtime *balanceRuntime, source model.BalanceDisk, disks []model.BalanceDisk, selected []string, opts model.BalanceOptions) {
	files, totalBytes, scanErr := l.scanBalance(ctx, runtime, source, selected)
	if scanErr != nil {
		l.finishBalance(runtime, "failed", scanErr.Error())
		return
	}
	l.balanceMu.Lock()
	runtime.job.TotalBytes = totalBytes
	runtime.job.TotalFiles = len(files)
	runtime.job.Phase = "copying"
	runtime.startedCopy = time.Now()
	l.balanceMu.Unlock()

	fatal := ""
	for _, file := range files {
		if ctx.Err() != nil {
			l.finishBalance(runtime, "cancelled", "")
			return
		}
		destination, ok := chooseBalanceDestination(runtime, disks, runtime.job.Destinations, file.Size)
		if !ok {
			fatal = "no destination has enough free space for " + file.Rel
			l.addBalanceError(runtime, fatal)
			break
		}
		destinationDisk, _ := balanceDiskByName(disks, destination)
		destinationPath := filepath.Join(destinationDisk.Mount, filepath.FromSlash(file.Rel))
		status, err := l.moveBalanceFile(ctx, runtime, source, file, destinationDisk, destinationPath, opts)
		l.recordBalanceFile(runtime, file, destination, status, err)
		if err != nil && status == "failed" {
			l.addBalanceError(runtime, file.Rel+": "+err.Error())
		}
		if ctx.Err() != nil {
			l.finishBalance(runtime, "cancelled", "")
			return
		}
	}

	l.balanceMu.Lock()
	runtime.job.Phase = "cleaning"
	l.balanceMu.Unlock()
	l.cleanBalanceDirectories(source, selected)

	state := "completed"
	if fatal != "" {
		state = "failed"
	}
	l.finishBalance(runtime, state, fatal)
}

func (l *Live) scanBalance(ctx context.Context, runtime *balanceRuntime, source model.BalanceDisk, selected []string) ([]balanceFile, uint64, error) {
	files := make([]balanceFile, 0, 1024)
	var total uint64
	for _, relative := range selected {
		root := filepath.Join(source.Mount, filepath.FromSlash(relative))
		if !withinMount(source.Mount, root) {
			return nil, 0, errors.New("path escapes the source disk")
		}
		err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if walkErr != nil {
				l.addBalanceError(runtime, current+": "+walkErr.Error())
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			rel, err := filepath.Rel(source.Mount, current)
			if err != nil {
				return nil
			}
			file := balanceFile{
				Rel: filepath.ToSlash(rel), Mode: info.Mode(), ModTime: info.ModTime(),
			}
			if info.Mode()&os.ModeSymlink != 0 {
				link, err := os.Readlink(current)
				if err != nil {
					return nil
				}
				file.Link = link
			} else if info.Mode().IsRegular() {
				file.Size = safeUint64(info.Size())
				if stat, ok := info.Sys().(*syscall.Stat_t); ok {
					file.Inode = stat.Ino
					file.Device = safeUint64(int64(stat.Dev))
				}
			} else {
				return nil
			}
			files = append(files, file)
			total += file.Size
			if len(files)%200 == 0 {
				l.balanceMu.Lock()
				runtime.job.CurrentPath = file.Rel
				runtime.job.TotalFiles = len(files)
				runtime.job.TotalBytes = total
				l.balanceMu.Unlock()
			}
			return nil
		})
		if err != nil && ctx.Err() == nil {
			return nil, total, err
		}
	}
	return files, total, nil
}

func chooseBalanceDestination(runtime *balanceRuntime, disks []model.BalanceDisk, requested []string, size uint64) (string, bool) {
	candidates := requested
	if len(candidates) == 0 {
		candidates = make([]string, 0, len(disks))
		for _, disk := range disks {
			candidates = append(candidates, disk.Name)
		}
	}
	best := ""
	var bestFree uint64
	for _, name := range candidates {
		if name == runtime.job.Source {
			continue
		}
		free, ok := runtime.destFree[name]
		if !ok || free < size {
			continue
		}
		if best == "" || free > bestFree {
			best, bestFree = name, free
		}
	}
	return best, best != ""
}

func (l *Live) moveBalanceFile(ctx context.Context, runtime *balanceRuntime, source model.BalanceDisk, file balanceFile, destination model.BalanceDisk, destinationPath string, opts model.BalanceOptions) (string, error) {
	sourcePath := filepath.Join(source.Mount, filepath.FromSlash(file.Rel))
	if !withinMount(source.Mount, sourcePath) || !withinMount(destination.Mount, destinationPath) {
		return "failed", errors.New("path escapes the disk")
	}
	l.balanceMu.Lock()
	runtime.job.CurrentPath = file.Rel
	l.balanceMu.Unlock()

	if opts.DryRun {
		return "planned", nil
	}
	if info, err := os.Lstat(destinationPath); err == nil {
		if !opts.Overwrite {
			return "skipped", fmt.Errorf("destination already exists (%d bytes)", info.Size())
		}
		if err := os.RemoveAll(destinationPath); err != nil {
			return "failed", err
		}
	} else if !os.IsNotExist(err) {
		return "failed", err
	}
	// #nosec G301 -- target share directory keeps the share permissions.
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return "failed", err
	}

	if file.Link != "" {
		if err := os.Symlink(file.Link, destinationPath); err != nil {
			return "failed", err
		}
	} else if existing, ok := hardlinkKey(runtime, file); ok {
		if err := os.Link(existing, destinationPath); err != nil {
			if err := copyBalanceFile(sourcePath, destinationPath, file); err != nil {
				return "failed", err
			}
			hardlinkStore(runtime, file, destinationPath)
		}
	} else {
		if err := copyBalanceFile(sourcePath, destinationPath, file); err != nil {
			return "failed", err
		}
		hardlinkStore(runtime, file, destinationPath)
	}

	if file.Link == "" {
		info, err := os.Stat(destinationPath)
		if err != nil {
			return "failed", err
		}
		if safeUint64(info.Size()) != file.Size {
			_ = os.Remove(destinationPath)
			return "failed", fmt.Errorf("size mismatch after copy (%d != %d)", info.Size(), file.Size)
		}
	}
	if opts.DeleteSource {
		if err := os.Remove(sourcePath); err != nil {
			return "failed", fmt.Errorf("remove source: %w", err)
		}
	}
	l.balanceMu.Lock()
	runtime.destFree[destination.Name] -= minUint64(runtime.destFree[destination.Name], file.Size)
	l.balanceMu.Unlock()
	return "moved", nil
}

func hardlinkKey(runtime *balanceRuntime, file balanceFile) (string, bool) {
	if file.Inode == 0 {
		return "", false
	}
	key := fmt.Sprintf("%d:%d", file.Device, file.Inode)
	existing, ok := runtime.hardlinks[key]
	return existing, ok
}

func hardlinkStore(runtime *balanceRuntime, file balanceFile, destination string) {
	if file.Inode == 0 {
		return
	}
	key := fmt.Sprintf("%d:%d", file.Device, file.Inode)
	if _, ok := runtime.hardlinks[key]; !ok {
		runtime.hardlinks[key] = destination
	}
}

func copyBalanceFile(source, destination string, file balanceFile) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode.Perm())
	if err != nil {
		return err
	}
	buffer := make([]byte, 1<<20)
	_, copyErr := io.CopyBuffer(output, input, buffer)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(destination)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(destination)
		return closeErr
	}
	_ = os.Chmod(destination, file.Mode.Perm())
	_ = os.Chtimes(destination, file.ModTime, file.ModTime)
	if stat, err := os.Stat(source); err == nil {
		if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
			_ = os.Chown(destination, int(sys.Uid), int(sys.Gid))
		}
	}
	return nil
}

func (l *Live) recordBalanceFile(runtime *balanceRuntime, file balanceFile, destination, status string, moveErr error) {
	l.balanceMu.Lock()
	defer l.balanceMu.Unlock()
	entry := model.BalanceMove{Path: file.Rel, Destination: destination, Size: file.Size, Status: status}
	if moveErr != nil {
		entry.Error = moveErr.Error()
	}
	if len(runtime.job.Files) < 5000 {
		runtime.job.Files = append(runtime.job.Files, entry)
	}
	if status == "moved" || status == "planned" {
		runtime.job.DoneBytes += file.Size
		runtime.job.DoneFiles++
	}
	elapsed := time.Since(runtime.startedCopy).Seconds()
	if elapsed > 0 {
		runtime.job.Speed = float64(runtime.job.DoneBytes) / elapsed
		remaining := uint64(0)
		if runtime.job.TotalBytes > runtime.job.DoneBytes {
			remaining = runtime.job.TotalBytes - runtime.job.DoneBytes
		}
		if runtime.job.Speed > 0 {
			runtime.job.ETA = int64(float64(remaining) / runtime.job.Speed)
		}
	}
	runtime.log = append(runtime.log, fmt.Sprintf("%s → %s (%s)", file.Rel, destination, status))
	if len(runtime.log) > 200 {
		runtime.log = runtime.log[len(runtime.log)-200:]
	}
	runtime.job.Log = append([]string(nil), runtime.log...)
}

func (l *Live) addBalanceError(runtime *balanceRuntime, message string) {
	l.balanceMu.Lock()
	defer l.balanceMu.Unlock()
	runtime.job.Errors = append(runtime.job.Errors, message)
	if len(runtime.job.Errors) > 100 {
		runtime.job.Errors = runtime.job.Errors[len(runtime.job.Errors)-100:]
	}
}

func (l *Live) cleanBalanceDirectories(source model.BalanceDisk, selected []string) {
	for _, relative := range selected {
		root := filepath.Join(source.Mount, filepath.FromSlash(relative))
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		directories := make([]string, 0)
		_ = filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
			if err == nil && entry.IsDir() && current != root {
				directories = append(directories, current)
			}
			return nil
		})
		sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
		for _, directory := range directories {
			_ = os.Remove(directory)
		}
	}
}

func (l *Live) finishBalance(runtime *balanceRuntime, state, message string) {
	if message != "" {
		l.addBalanceError(runtime, message)
	}
	l.balanceMu.Lock()
	runtime.job.State = state
	runtime.job.Phase = "done"
	runtime.job.CurrentPath = ""
	runtime.job.Finished = time.Now().UTC()
	if state == "completed" && runtime.job.TotalFiles > 0 {
		runtime.job.DoneBytes = runtime.job.TotalBytes
	}
	finished := snapshotBalance(runtime.job)
	runtime.cancel()
	l.balanceMu.Unlock()
	if state == "completed" || state == "failed" || state == "cancelled" {
		l.balanceMu.Lock()
		l.balanceHistory = append([]model.BalanceJob{finished}, l.balanceHistory...)
		if len(l.balanceHistory) > 20 {
			l.balanceHistory = l.balanceHistory[:20]
		}
		l.balanceMu.Unlock()
		l.saveBalanceHistory()
	}
}

func (l *Live) saveBalanceHistory() {
	if l.configDir() == "" {
		return
	}
	l.balanceMu.Lock()
	history := make([]model.BalanceJob, 0, len(l.balanceHistory))
	for _, job := range l.balanceHistory {
		copied := snapshotBalance(job)
		if len(copied.Files) > 2000 {
			copied.Files = copied.Files[:2000]
		}
		history = append(history, copied)
	}
	l.balanceMu.Unlock()
	raw, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return
	}
	_ = l.saveConfigFile("balance-history.json", raw)
}

func (l *Live) loadBalanceHistory() {
	if l.configDir() == "" {
		return
	}
	raw, err := l.readConfigFile("balance-history.json")
	if err != nil {
		return
	}
	var history []model.BalanceJob
	if json.Unmarshal(raw, &history) != nil {
		return
	}
	l.balanceMu.Lock()
	l.balanceHistory = history
	l.balanceMu.Unlock()
}
