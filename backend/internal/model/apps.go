package model

import "time"

// AppCatalogEntry is one Community Applications result. Docker applications
// expose TemplateURL, plugins expose PluginURL.
type AppCatalogEntry struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Kind          string   `json:"kind"`
	Repository    string   `json:"repository,omitempty"`
	Registry      string   `json:"registry,omitempty"`
	Network       string   `json:"network,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	WebUI         string   `json:"webUi,omitempty"`
	Overview      string   `json:"overview,omitempty"`
	Category      string   `json:"category,omitempty"`
	Categories    []string `json:"categories"`
	Support       string   `json:"support,omitempty"`
	Project       string   `json:"project,omitempty"`
	ReadMe        string   `json:"readMe,omitempty"`
	TemplateURL   string   `json:"templateUrl,omitempty"`
	PluginURL     string   `json:"pluginUrl,omitempty"`
	Author        string   `json:"author,omitempty"`
	Repo          string   `json:"repo,omitempty"`
	Date          string   `json:"date,omitempty"`
	Requires      string   `json:"requires,omitempty"`
	ExtraParams   string   `json:"extraParams,omitempty"`
	Privileged    bool     `json:"privileged"`
	Downloads     int      `json:"downloads"`
	PluginVersion string   `json:"pluginVersion,omitempty"`
}

// AppCatalogPage is a filtered page of the cached Community Applications feed.
type AppCatalogPage struct {
	Entries    []AppCatalogEntry `json:"entries"`
	Total      int               `json:"total"`
	Categories []string          `json:"categories"`
	UpdatedAt  *time.Time        `json:"updatedAt,omitempty"`
	Cached     bool              `json:"cached"`
}

// AppDetail adds the parsed Unraid template when the catalog entry is a
// Docker application.
type AppDetail struct {
	AppCatalogEntry
	Template *ContainerTemplate `json:"template,omitempty"`
}
