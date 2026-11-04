package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

const (
	caFeedURL       = "https://ca.unraid.net/assets/feed/applicationFeed-small.json"
	caFeedCacheName = "apps-feed.json"
	caFeedTTL       = 6 * time.Hour
	caFeedMaxSize   = 32 << 20
	caTemplateMax   = 2 << 20
)

type caCacheFile struct {
	FetchedAt time.Time       `json:"fetchedAt"`
	FeedAt    time.Time       `json:"feedAt"`
	Payload   json.RawMessage `json:"payload"`
}

// AppsCatalog returns a filtered page of the Community Applications feed.
// The feed is downloaded once, cached on the flash drive and kept in memory.
func (l *Live) AppsCatalog(ctx context.Context, query, kind, category, order string, limit, offset int) (model.AppCatalogPage, error) {
	if err := l.loadAppsCatalog(ctx, false); err != nil {
		return model.AppCatalogPage{}, err
	}
	l.appsMu.Lock()
	entries := append([]model.AppCatalogEntry(nil), l.appsEntries...)
	categories := append([]string(nil), l.appsCategories...)
	fetchedAt := l.appsFetchedAt
	l.appsMu.Unlock()

	needle := strings.ToLower(strings.TrimSpace(query))
	kind = strings.ToLower(strings.TrimSpace(kind))
	category = strings.ToLower(strings.TrimSpace(category))
	filtered := make([]model.AppCatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if kind != "" && kind != "all" && entry.Kind != kind {
			continue
		}
		if category != "" && category != "all" {
			found := strings.EqualFold(entry.Category, category)
			for _, item := range entry.Categories {
				if strings.EqualFold(item, category) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if needle != "" {
			haystack := strings.ToLower(strings.Join([]string{
				entry.Name, entry.Repository, entry.Overview, entry.Author, entry.Repo,
				strings.Join(entry.Categories, " "),
			}, " "))
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		filtered = append(filtered, entry)
	}

	switch strings.ToLower(strings.TrimSpace(order)) {
	case "name":
		sort.SliceStable(filtered, func(i, j int) bool { return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name) })
	case "newest":
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Date > filtered[j].Date })
	default:
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].Downloads != filtered[j].Downloads {
				return filtered[i].Downloads > filtered[j].Downloads
			}
			return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		})
	}

	total := len(filtered)
	if limit <= 0 || limit > 100 {
		limit = 60
	}
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return model.AppCatalogPage{
		Entries: filtered[offset:end], Total: total, Categories: categories,
		UpdatedAt: &fetchedAt, Cached: time.Since(fetchedAt) < caFeedTTL,
	}, nil
}

// AppTemplate resolves a Docker catalog entry and returns its parsed Unraid
// template. Only URLs present in the current feed are accepted.
func (l *Live) AppTemplate(ctx context.Context, templateURL string) (model.ContainerTemplate, error) {
	templateURL = strings.TrimSpace(templateURL)
	if err := validateHTTPSURL(templateURL); err != nil {
		return model.ContainerTemplate{}, err
	}
	if err := l.loadAppsCatalog(ctx, false); err != nil {
		return model.ContainerTemplate{}, err
	}
	l.appsMu.Lock()
	found := false
	for _, entry := range l.appsEntries {
		if entry.TemplateURL == templateURL {
			found = true
			break
		}
	}
	l.appsMu.Unlock()
	if !found {
		return model.ContainerTemplate{}, errors.New("this template does not belong to a catalog entry")
	}

	raw, err := l.fetchCABytes(ctx, templateURL, caTemplateMax)
	if err != nil {
		return model.ContainerTemplate{}, fmt.Errorf("download template: %w", err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		return model.ContainerTemplate{}, fmt.Errorf("parse template: %w", err)
	}
	template := templateFromXML(root)
	if template.Name == "" || template.Image == "" {
		return model.ContainerTemplate{}, errors.New("the catalog template is incomplete")
	}
	template.ID = "ca:" + templateURL
	return template, nil
}

// AppIcon proxies a catalog icon through Aurora so the SPA keeps a strict
// img-src CSP and the browser never talks to community hosts directly.
func (l *Live) AppIcon(ctx context.Context, iconURL string) ([]byte, string, error) {
	iconURL = strings.TrimSpace(iconURL)
	if err := validateHTTPSURL(iconURL); err != nil {
		return nil, "", err
	}
	if err := l.loadAppsCatalog(ctx, false); err != nil {
		return nil, "", err
	}
	l.appsMu.Lock()
	found := false
	for _, entry := range l.appsEntries {
		if entry.Icon == iconURL {
			found = true
			break
		}
	}
	l.appsMu.Unlock()
	if !found {
		return nil, "", errors.New("this icon does not belong to a catalog entry")
	}
	raw, err := l.fetchCABytes(ctx, iconURL, 2<<20)
	if err != nil {
		return nil, "", fmt.Errorf("download icon: %w", err)
	}
	contentType := http.DetectContentType(raw)
	if !strings.HasPrefix(contentType, "image/") {
		trimmed := strings.TrimSpace(string(raw))
		if strings.Contains(trimmed, "<svg") || strings.HasPrefix(trimmed, "<?xml") {
			contentType = "image/svg+xml"
		} else {
			return nil, "", errors.New("the catalog icon is not an image")
		}
	}
	return raw, contentType, nil
}

func (l *Live) loadAppsCatalog(ctx context.Context, force bool) error {
	l.appsMu.Lock()
	defer l.appsMu.Unlock()
	if !force && len(l.appsEntries) > 0 && time.Since(l.appsFetchedAt) < caFeedTTL {
		return nil
	}

	cachePath := l.configPath(caFeedCacheName)
	if !force {
		if raw, err := os.ReadFile(cachePath); err == nil {
			var cache caCacheFile
			if json.Unmarshal(raw, &cache) == nil && len(cache.Payload) > 0 && time.Since(cache.FetchedAt) < caFeedTTL {
				entries, categories, feedAt, err := parseCAFeed(cache.Payload)
				if err == nil {
					l.appsEntries, l.appsCategories = entries, categories
					l.appsFetchedAt, l.appsFeedAt = cache.FetchedAt, feedAt
					return nil
				}
			}
		}
	}

	payload, err := l.fetchCABytes(ctx, caFeedURL, caFeedMaxSize)
	if err != nil {
		slog.Warn("download Community Applications feed", "error", err)
		if len(l.appsEntries) > 0 {
			return nil
		}
		return fmt.Errorf("download Community Applications feed: %w", err)
	}
	entries, categories, feedAt, err := parseCAFeed(payload)
	if err != nil {
		return err
	}
	now := time.Now()
	l.appsEntries, l.appsCategories = entries, categories
	l.appsFetchedAt, l.appsFeedAt = now, feedAt
	cache := caCacheFile{FetchedAt: now, FeedAt: feedAt, Payload: payload}
	if raw, err := json.Marshal(cache); err == nil {
		_ = writeFileAtomic(cachePath, raw, 0o600)
	}
	return nil
}

func (l *Live) fetchCABytes(ctx context.Context, target string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Aurora")
	client := l.webHTTPClient()
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	clone := *client
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return errors.New("only HTTPS redirects are accepted")
		}
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return nil
	}
	response, err := clone.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("response is too large")
	}
	return raw, nil
}

func parseCAFeed(raw []byte) ([]model.AppCatalogEntry, []string, time.Time, error) {
	var payload struct {
		LastUpdated string `json:"last_updated"`
		Timestamp   int64  `json:"last_updated_timestamp"`
		AppList     []any  `json:"applist"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, nil, time.Time{}, fmt.Errorf("decode Community Applications feed: %w", err)
	}
	entries := make([]model.AppCatalogEntry, 0, len(payload.AppList))
	categorySet := map[string]bool{}
	for _, item := range payload.AppList {
		app, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(jsonString(app["Name"]))
		if name == "" {
			continue
		}
		templateURL := strings.TrimSpace(jsonString(app["TemplateURL"]))
		pluginURL := strings.TrimSpace(jsonString(app["PluginURL"]))
		kind := "docker"
		if jsonBool(app["Plugin"]) || pluginURL != "" {
			kind = "plugin"
		}
		if kind == "docker" && templateURL == "" {
			continue
		}
		if kind == "plugin" && pluginURL == "" {
			continue
		}
		categories := jsonStrings(app["CategoryList"])
		for _, category := range categories {
			if category = strings.TrimSpace(category); category != "" {
				categorySet[category] = true
			}
		}
		category := ""
		if len(categories) > 0 {
			category = categories[0]
		}
		id := templateURL
		if kind == "plugin" {
			id = pluginURL
		}
		entry := model.AppCatalogEntry{
			ID:            id,
			Name:          name,
			Kind:          kind,
			Repository:    jsonString(app["Repository"]),
			Registry:      jsonString(app["Registry"]),
			Network:       jsonString(app["Network"]),
			Icon:          jsonString(app["Icon"]),
			WebUI:         jsonString(app["WebUI"]),
			Overview:      jsonString(app["Overview"]),
			Category:      category,
			Categories:    categories,
			Support:       jsonString(app["Support"]),
			Project:       jsonString(app["Project"]),
			ReadMe:        jsonString(app["ReadMe"]),
			TemplateURL:   templateURL,
			PluginURL:     pluginURL,
			Author:        firstNonEmpty(jsonString(app["Author"]), jsonString(app["PluginAuthor"])),
			Repo:          jsonString(app["Repo"]),
			Date:          jsonString(app["Date"]),
			Requires:      jsonString(app["Requires"]),
			ExtraParams:   jsonString(app["ExtraParams"]),
			Privileged:    jsonBool(app["Privileged"]),
			Downloads:     jsonInt(app["downloads"]),
			PluginVersion: jsonString(app["pluginVersion"]),
		}
		entries = append(entries, entry)
	}
	categories := make([]string, 0, len(categorySet))
	for category := range categorySet {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	feedAt := time.Now()
	if payload.Timestamp > 0 {
		feedAt = time.Unix(payload.Timestamp, 0)
	} else if payload.LastUpdated != "" {
		if parsed, err := time.Parse(time.RFC3339, payload.LastUpdated); err == nil {
			feedAt = parsed
		}
	}
	return entries, categories, feedAt, nil
}

func validateHTTPSURL(target string) error {
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("only HTTPS URLs are accepted")
	}
	if parsed.User != nil {
		return errors.New("URL credentials are not accepted")
	}
	return nil
}

func jsonString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func jsonBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	default:
		return false
	}
}

func jsonInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func jsonStrings(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if text := strings.TrimSpace(jsonString(item)); text != "" {
			out = append(out, text)
		}
	}
	return out
}
