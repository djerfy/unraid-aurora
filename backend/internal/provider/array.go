package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"aurora/internal/model"
)

var printableKey = regexp.MustCompile(`^[ -~]+$`)

func (l *Live) StartArray(ctx context.Context, opts model.ArrayStartOptions) error {
	values, err := buildArrayStartForm(opts)
	if err != nil {
		return err
	}
	varIni, _ := parseINI(filepath.Join(l.stateDir, "var.ini"))
	if strings.EqualFold(varIni.get("", "mdState"), "STARTED") {
		return fmt.Errorf("array is already started")
	}
	if opts.KeyType == "keyfile" {
		keyfile := strings.TrimSpace(varIni.get("", "luksKeyfile"))
		if keyfile == "" {
			return fmt.Errorf("no LUKS keyfile path is configured")
		}
		data, err := base64.StdEncoding.DecodeString(opts.Key)
		if err != nil {
			return fmt.Errorf("decode keyfile: %w", err)
		}
		if len(data) == 0 || len(data) > 1024*1024 {
			return fmt.Errorf("keyfile must be between 1 byte and 1 MiB")
		}
		if err := os.MkdirAll(filepath.Dir(keyfile), 0o700); err != nil && !os.IsExist(err) {
			return fmt.Errorf("create keyfile directory: %w", err)
		}
		if err := os.WriteFile(keyfile, data, 0o600); err != nil {
			return fmt.Errorf("write keyfile: %w", err)
		}
	}
	if err := runEmcmd(ctx, values.Encode()); err != nil {
		return fmt.Errorf("start array: %w", err)
	}
	return nil
}

func (l *Live) StopArray(ctx context.Context) error {
	if err := runEmcmd(ctx, "cmdStop=Stop"); err != nil {
		return fmt.Errorf("stop array: %w", err)
	}
	return nil
}

func ValidateArrayStartOptions(opts model.ArrayStartOptions) error {
	_, err := buildArrayStartForm(opts)
	return err
}

func buildArrayStartForm(opts model.ArrayStartOptions) (url.Values, error) {
	values := url.Values{"cmdStart": {"Start"}}
	switch opts.Mode {
	case "", "normal":
	case "maintenance":
		values.Set("startMode", "Maintenance")
	default:
		return nil, fmt.Errorf("unsupported array mode %q", opts.Mode)
	}
	switch opts.KeyType {
	case "", "none":
	case "passphrase":
		if opts.Key == "" || len(opts.Key) > 512 || !printableKey.MatchString(opts.Key) {
			return nil, fmt.Errorf("passphrase must use 1-512 printable ASCII characters")
		}
		values.Set("luksKey", base64.StdEncoding.EncodeToString([]byte(opts.Key)))
	case "keyfile":
		if opts.Key == "" {
			return nil, fmt.Errorf("keyfile content is required")
		}
	default:
		return nil, fmt.Errorf("unsupported key type %q", opts.KeyType)
	}
	return values, nil
}

func runEmcmd(ctx context.Context, payload string) error {
	emcmd := "/usr/local/sbin/emcmd"
	if info, err := os.Stat(emcmd); err != nil || info.IsDir() {
		return fmt.Errorf("%s is not available", emcmd)
	}
	out, err := exec.CommandContext(ctx, emcmd, payload).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return fmt.Errorf("%s", msg)
	}
	return nil
}
