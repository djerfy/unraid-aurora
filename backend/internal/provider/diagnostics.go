package provider

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	diagnosticsFileLimit = 2 << 20
	diagnosticsTailLimit = 512 << 10
)

// Diagnostics builds a zip archive similar to the official Unraid Tools >
// Diagnostics page: host identity, config files, command output and log tails.
func (l *Live) Diagnostics(ctx context.Context) ([]byte, error) {
	buffer := &bytes.Buffer{}
	archive := zip.NewWriter(buffer)

	write := func(name string, data []byte) {
		if len(data) > diagnosticsFileLimit {
			data = data[len(data)-diagnosticsFileLimit:]
		}
		entry, err := archive.Create(name)
		if err != nil {
			return
		}
		_, _ = entry.Write(data)
	}
	writeText := func(name string, text string) { write(name, []byte(text)) }

	writeText("README.txt", fmt.Sprintf(
		"Aurora diagnostics\nGenerated: %s\nHostname: %s\n",
		time.Now().Format(time.RFC3339), hostname(),
	))

	l.writeDiagnosticsFiles(write, "state", l.stateDir, ".ini")
	l.writeDiagnosticsFiles(write, "boot", filepath.Join(l.bootDir, "config"), ".cfg")
	l.writeDiagnosticsFiles(write, "data", l.dataDir, ".json")

	commands := []struct {
		name string
		args []string
	}{
		{"system/uname.txt", []string{"uname", "-a"}},
		{"system/uptime.txt", []string{"uptime"}},
		{"system/df.txt", []string{"df", "-h"}},
		{"system/free.txt", []string{"free", "-m"}},
		{"system/lspci.txt", []string{"lspci", "-nnk"}},
		{"system/lsusb.txt", []string{"lsusb"}},
		{"system/ip-addr.txt", []string{"ip", "addr"}},
		{"system/ip-route.txt", []string{"ip", "route"}},
		{"docker/containers.txt", []string{"/usr/bin/docker", "ps", "-a", "--no-trunc"}},
		{"docker/images.txt", []string{"/usr/bin/docker", "images"}},
		{"docker/networks.txt", []string{"/usr/bin/docker", "network", "ls"}},
		{"vms/list.txt", []string{"virsh", "list", "--all"}},
	}
	for _, command := range commands {
		writeText(command.name, runDiagnosticsCommand(ctx, command.args))
	}

	writeText("plugins/installed.txt", installedPlugins())
	writeText("logs/syslog.txt", tailFile("/var/log/syslog", diagnosticsTailLimit))
	writeText("logs/docker.txt", tailFile("/var/log/docker.log", diagnosticsTailLimit/2))
	writeText("logs/libvirt.txt", tailFile("/var/log/libvirt/libvirtd.log", diagnosticsTailLimit/2))

	if err := archive.Close(); err != nil {
		return nil, fmt.Errorf("close diagnostics archive: %w", err)
	}
	return buffer.Bytes(), nil
}

func (l *Live) writeDiagnosticsFiles(write func(string, []byte), prefix, directory, extension string) {
	if directory == "" {
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), extension) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Size() > diagnosticsFileLimit {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			continue
		}
		write(prefix+"/"+entry.Name(), data)
	}
}

func runDiagnosticsCommand(ctx context.Context, argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	commandCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(commandCtx, argv[0], argv[1:]...).CombinedOutput()
	header := "$ " + strings.Join(argv, " ") + "\n"
	if err != nil {
		return header + string(output) + "\n(error: " + err.Error() + ")\n"
	}
	return header + string(output) + "\n"
}

func tailFile(path string, limit int) string {
	handle, err := os.Open(path)
	if err != nil {
		return "(unavailable: " + err.Error() + ")\n"
	}
	defer handle.Close()
	info, err := handle.Stat()
	if err != nil {
		return "(unavailable: " + err.Error() + ")\n"
	}
	offset := info.Size() - int64(limit)
	if offset < 0 {
		offset = 0
	}
	if _, err := handle.Seek(offset, io.SeekStart); err != nil {
		return "(unavailable: " + err.Error() + ")\n"
	}
	data, err := io.ReadAll(io.LimitReader(handle, int64(limit)))
	if err != nil {
		return "(unavailable: " + err.Error() + ")\n"
	}
	return string(data)
}

func installedPlugins() string {
	entries, err := os.ReadDir("/var/log/plugins")
	if err != nil {
		return "(unavailable: " + err.Error() + ")\n"
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return strings.Join(names, "\n") + "\n"
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}
