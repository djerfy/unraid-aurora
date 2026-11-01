package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aurora/internal/model"
)

var (
	shareNamePattern    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,39}$`)
	shareFloorPattern   = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?[KMGTP]?$`)
	sharePoolPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	shareDiskPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	shareHostListMaxLen = 4096
)

func (l *Live) shareConfigPath(name string) string {
	return filepath.Join(l.bootDir, "config", "shares", name+".cfg")
}

func (l *Live) ShareConfig(ctx context.Context, name string) (model.ShareConfig, error) {
	if !shareNamePattern.MatchString(name) {
		return model.ShareConfig{}, errors.New("invalid share name")
	}
	shares, err := parseINI(filepath.Join(l.stateDir, "shares.ini"))
	if err != nil {
		return model.ShareConfig{}, err
	}
	section, ok := shares[name]
	if !ok {
		return model.ShareConfig{}, fmt.Errorf("share %q not found", name)
	}
	cfg := model.ShareConfig{
		Name: name, Comment: section["comment"], Floor: section["floor"],
		Allocator: section["allocator"], SplitLevel: section["splitLevel"],
		UseCache: section["useCache"], CachePool: section["cachePool"],
		CachePool2: section["cachePool2"], COW: section["cow"],
		Exclusive: section["exclusive"] == "yes",
		Include:   splitShareList(section["include"]),
		Exclude:   splitShareList(section["exclude"]),
		SMBExport: "-", SMBSecurity: "public", SMBCaseSensitive: "auto",
		NFSExport: "-", NFSSecurity: "public",
	}
	if cfg.Allocator == "" {
		cfg.Allocator = "highwater"
	}
	if cfg.UseCache == "" {
		cfg.UseCache = "no"
	}
	if cfg.COW == "" {
		cfg.COW = "auto"
	}
	if cfg.Floor == "" {
		cfg.Floor = "0"
	}
	applyShareSMBConfig(&cfg, l.stateDir)
	applyShareNFSConfig(&cfg, l.stateDir)
	return cfg, nil
}

func splitShareList(value string) []string {
	items := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func applyShareSMBConfig(cfg *model.ShareConfig, stateDir string) {
	sec, err := parseINI(filepath.Join(stateDir, "sec.ini"))
	if err != nil {
		return
	}
	section, ok := sec[cfg.Name]
	if !ok {
		return
	}
	if value := section["export"]; value != "" {
		cfg.SMBExport = value
	}
	if value := section["security"]; value != "" {
		cfg.SMBSecurity = value
	}
	if value := section["caseSensitive"]; value != "" {
		cfg.SMBCaseSensitive = value
	}
	cfg.SMBVolumeSizeLimit = section["volsizelimit"]
}

func applyShareNFSConfig(cfg *model.ShareConfig, stateDir string) {
	sec, err := parseINI(filepath.Join(stateDir, "sec_nfs.ini"))
	if err != nil {
		return
	}
	section, ok := sec[cfg.Name]
	if !ok {
		return
	}
	if value := section["export"]; value != "" {
		cfg.NFSExport = value
	}
	if value := section["security"]; value != "" {
		cfg.NFSSecurity = value
	}
	cfg.NFSHostList = section["hostList"]
}

func validateShareConfig(cfg model.ShareConfig, name string) error {
	if cfg.Name != name || !shareNamePattern.MatchString(name) {
		return errors.New("share name cannot be changed here")
	}
	if len(cfg.Comment) > 256 || strings.ContainsAny(cfg.Comment, "\x00\n\r") {
		return errors.New("comment must be a single line of 256 characters or fewer")
	}
	if cfg.Floor != "" && !shareFloorPattern.MatchString(strings.ToUpper(cfg.Floor)) {
		return errors.New("minimum free space must be a size such as 0, 10G or 500M")
	}
	switch cfg.Allocator {
	case "highwater", "fillup", "mostfree":
	default:
		return fmt.Errorf("unsupported allocation method %q", cfg.Allocator)
	}
	switch cfg.SplitLevel {
	case "", "0", "1", "2", "3", "4", "5":
	default:
		return fmt.Errorf("unsupported split level %q", cfg.SplitLevel)
	}
	switch cfg.UseCache {
	case "no", "yes", "prefer", "only":
	default:
		return fmt.Errorf("unsupported cache mode %q", cfg.UseCache)
	}
	switch cfg.COW {
	case "", "auto", "no":
	default:
		return fmt.Errorf("unsupported copy-on-write mode %q", cfg.COW)
	}
	if cfg.CachePool != "" && !sharePoolPattern.MatchString(cfg.CachePool) {
		return errors.New("invalid primary pool")
	}
	if cfg.CachePool2 != "" && !sharePoolPattern.MatchString(cfg.CachePool2) {
		return errors.New("invalid secondary pool")
	}
	for _, values := range [][]string{cfg.Include, cfg.Exclude} {
		for _, value := range values {
			if !shareDiskPattern.MatchString(value) {
				return fmt.Errorf("invalid disk %q", value)
			}
		}
	}
	return nil
}

func validateShareSMB(cfg model.ShareConfig) error {
	switch cfg.SMBExport {
	case "-", "e", "eh", "et", "eth":
	default:
		return fmt.Errorf("unsupported SMB export %q", cfg.SMBExport)
	}
	switch cfg.SMBSecurity {
	case "public", "secure", "private":
	default:
		return fmt.Errorf("unsupported SMB security %q", cfg.SMBSecurity)
	}
	switch cfg.SMBCaseSensitive {
	case "", "auto", "yes", "forced":
	default:
		return fmt.Errorf("unsupported case sensitivity %q", cfg.SMBCaseSensitive)
	}
	if cfg.SMBVolumeSizeLimit != "" && !regexp.MustCompile(`^[0-9]+$`).MatchString(cfg.SMBVolumeSizeLimit) {
		return errors.New("Time Machine volume size must be a number of megabytes")
	}
	return nil
}

func validateShareNFS(cfg model.ShareConfig) error {
	switch cfg.NFSExport {
	case "-", "e":
	default:
		return fmt.Errorf("unsupported NFS export %q", cfg.NFSExport)
	}
	switch cfg.NFSSecurity {
	case "public", "secure", "private":
	default:
		return fmt.Errorf("unsupported NFS security %q", cfg.NFSSecurity)
	}
	if len(cfg.NFSHostList) > shareHostListMaxLen || strings.ContainsAny(cfg.NFSHostList, "\x00") {
		return errors.New("NFS host list is too long or contains invalid characters")
	}
	return nil
}

func (l *Live) captureShareConfig(name string) []byte {
	data, err := os.ReadFile(l.shareConfigPath(name))
	if err != nil {
		return nil
	}
	return data
}

func (l *Live) finishShareChange(action, description, name string, before []byte) {
	after, err := os.ReadFile(l.shareConfigPath(name))
	if err != nil {
		return
	}
	l.recordExternalConfigChange(action, description, l.shareConfigPath(name), before, after)
}

func (l *Live) UpdateShareProperties(ctx context.Context, name string, cfg model.ShareConfig) error {
	if err := validateShareConfig(cfg, name); err != nil {
		return err
	}
	before := l.captureShareConfig(name)
	include := strings.Join(cfg.Include, ",")
	exclude := strings.Join(cfg.Exclude, ",")
	values := url.Values{
		"cmdEditShare":     {"Apply"},
		"shareNameOrig":    {name},
		"shareName":        {name},
		"shareComment":     {cfg.Comment},
		"shareFloor":       {cfg.Floor},
		"shareCachePool":   {cfg.CachePool},
		"shareCachePool2":  {cfg.CachePool2},
		"shareUseCache":    {cfg.UseCache},
		"shareCOW":         {cfg.COW},
		"shareAllocator":   {cfg.Allocator},
		"shareAllocator1":  {cfg.Allocator},
		"shareAllocator2":  {cfg.Allocator},
		"shareSplitLevel":  {cfg.SplitLevel},
		"shareSplitLevel1": {cfg.SplitLevel},
		"shareSplitLevel2": {cfg.SplitLevel},
		"shareInclude":     {include},
		"shareInclude1":    {include},
		"shareInclude2":    {include},
		"shareExclude":     {exclude},
		"shareExclude1":    {exclude},
		"shareExclude2":    {exclude},
	}
	if err := runEmcmd(ctx, values.Encode()); err != nil {
		return fmt.Errorf("update share: %w", err)
	}
	l.finishShareChange("share.update", "Update share settings · "+name, name, before)
	return nil
}

func (l *Live) UpdateShareSMB(ctx context.Context, name string, cfg model.ShareConfig) error {
	if !shareNamePattern.MatchString(name) {
		return errors.New("invalid share name")
	}
	if err := validateShareSMB(cfg); err != nil {
		return err
	}
	before := l.captureShareConfig(name)
	values := url.Values{
		"changeShareSecurity": {"Apply"},
		"shareName":           {name},
		"shareExport":         {cfg.SMBExport},
		"shareSecurity":       {cfg.SMBSecurity},
		"shareCaseSensitive":  {cfg.SMBCaseSensitive},
		"shareVolsizelimit":   {cfg.SMBVolumeSizeLimit},
	}
	if err := runEmcmd(ctx, values.Encode()); err != nil {
		return fmt.Errorf("update SMB share: %w", err)
	}
	l.finishShareChange("share.smb", "Update SMB settings · "+name, name, before)
	return nil
}

func (l *Live) UpdateShareNFS(ctx context.Context, name string, cfg model.ShareConfig) error {
	if !shareNamePattern.MatchString(name) {
		return errors.New("invalid share name")
	}
	if err := validateShareNFS(cfg); err != nil {
		return err
	}
	before := l.captureShareConfig(name)
	values := url.Values{
		"changeShareSecurityNFS": {"Apply"},
		"shareName":              {name},
		"shareExportNFS":         {cfg.NFSExport},
		"shareSecurityNFS":       {cfg.NFSSecurity},
	}
	if err := runEmcmd(ctx, values.Encode()); err != nil {
		return fmt.Errorf("update NFS share: %w", err)
	}
	if cfg.NFSSecurity == "private" {
		hostValues := url.Values{
			"changeShareAccessNFS": {"Apply"},
			"shareName":            {name},
			"shareHostListNFS":     {cfg.NFSHostList},
		}
		if err := runEmcmd(ctx, hostValues.Encode()); err != nil {
			return fmt.Errorf("update NFS rules: %w", err)
		}
	}
	l.finishShareChange("share.nfs", "Update NFS settings · "+name, name, before)
	return nil
}
