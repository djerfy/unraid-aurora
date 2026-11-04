package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurora/internal/redact"
)

var configImportFiles = map[string]bool{
	"settings.json":       true,
	"storage-layout.json": true,
	"mover-history.json":  true,
	"audit-log.json":      true,
	"image-updates.json":  true,
}

func (s *Server) flashBackup(w http.ResponseWriter, r *http.Request) {
	s.writeZip(w, "aurora-flash-backup", s.cfg.BootDir, func(rel string) bool {
		// Trivy is large and regenerable; keep it out of flash backups.
		return !strings.HasPrefix(filepath.ToSlash(rel), "config/aurora/trivy/")
	}, false)
}

func (s *Server) configExport(w http.ResponseWriter, r *http.Request) {
	redacted := r.URL.Query().Get("redact") == "1"
	name := "aurora-config"
	if redacted {
		name = "aurora-config-redacted"
	}
	s.writeZip(w, name, s.cfg.ConfigDir, func(rel string) bool {
		if strings.HasPrefix(rel, "trivy/") || rel == "api-tokens.json" || rel == "metrics-archive.json" {
			return false
		}
		return configImportFiles[rel] || strings.HasSuffix(rel, ".json")
	}, redacted)
}

func (s *Server) writeZip(w http.ResponseWriter, name, root string, include func(string) bool, redactData bool) {
	if root == "" {
		writeError(w, http.StatusBadRequest, "empty_root", "directory is not configured")
		return
	}
	stamp := time.Now().Format("20060102-150405")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`-`+stamp+`.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "lost+found" || entry.Name() == "System Volume Information" {
				return filepath.SkipDir
			}
			return nil
		}
		if include != nil && !include(filepath.ToSlash(rel)) {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return nil
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		if redactData && strings.HasSuffix(strings.ToLower(rel), ".json") {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			header = &zip.FileHeader{Name: header.Name, Method: zip.Deflate, Modified: info.ModTime()}
			writer, err := zw.CreateHeader(header)
			if err != nil {
				return nil
			}
			_, _ = writer.Write(redact.JSON(data))
			return nil
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		_, _ = io.Copy(writer, io.LimitReader(file, 256<<20))
		_ = file.Close()
		return nil
	})
}

func (s *Server) configImport(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "a zip archive is required")
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_archive", err.Error())
		return
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_archive", err.Error())
		return
	}
	if err := os.MkdirAll(s.cfg.ConfigDir, 0o750); err != nil {
		s.fail(w, err)
		return
	}
	imported := []string{}
	for _, entry := range reader.File {
		name := filepath.ToSlash(entry.Name)
		if !configImportFiles[name] {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(rc, 8<<20))
		_ = rc.Close()
		if err != nil || !json.Valid(raw) {
			continue
		}
		target := filepath.Join(s.cfg.ConfigDir, name)
		tmp := target + ".import"
		if err := os.WriteFile(tmp, raw, 0o600); err != nil {
			continue
		}
		if err := os.Rename(tmp, target); err != nil {
			_ = os.Remove(tmp)
			continue
		}
		imported = append(imported, name)
	}
	if len(imported) == 0 {
		writeError(w, http.StatusBadRequest, "nothing_imported", "the archive contains no Aurora configuration file")
		return
	}
	s.log.Info("aurora config imported", "files", imported)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "files": imported, "restartRequired": true})
}
