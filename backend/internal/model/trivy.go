package model

import "time"

// ContainerVulnerability is the subset of a Trivy finding shown in the UI.
type ContainerVulnerability struct {
	ID               string `json:"id"`
	Package          string `json:"package"`
	InstalledVersion string `json:"installedVersion"`
	FixedVersion     string `json:"fixedVersion"`
	Severity         string `json:"severity"`
	Title            string `json:"title"`
	Target           string `json:"target"`
	Type             string `json:"type"`
	URL              string `json:"url"`
}

// ContainerSecurity is the summary attached to every container.
type ContainerSecurity struct {
	Enabled      bool       `json:"enabled"`
	Available    bool       `json:"available"`
	Scanning     bool       `json:"scanning"`
	Stale        bool       `json:"stale"`
	ScannedAt    *time.Time `json:"scannedAt,omitempty"`
	TrivyVersion string     `json:"trivyVersion,omitempty"`
	DBUpdatedAt  *time.Time `json:"dbUpdatedAt,omitempty"`
	Critical     int        `json:"critical"`
	High         int        `json:"high"`
	Medium       int        `json:"medium"`
	Low          int        `json:"low"`
	Unknown      int        `json:"unknown"`
	Total        int        `json:"total"`
	Error        string     `json:"error,omitempty"`
	Image        string     `json:"image,omitempty"`
	ImageID      string     `json:"imageId,omitempty"`
}

// ContainerSecurityDetail adds the finding list for the container drawer.
type ContainerSecurityDetail struct {
	ContainerSecurity
	Vulnerabilities []ContainerVulnerability `json:"vulnerabilities"`
}

// SecurityContainerRef identifies a container using a scanned image.
type SecurityContainerRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// SecurityImage merges every container sharing the same image into a single
// scan result, so the security overview does not duplicate findings.
type SecurityImage struct {
	Image           string                   `json:"image"`
	ImageID         string                   `json:"imageId,omitempty"`
	Containers      []SecurityContainerRef   `json:"containers"`
	Security        ContainerSecurity        `json:"security"`
	Vulnerabilities []ContainerVulnerability `json:"vulnerabilities"`
}

// SecurityTotals summarises the merged scan results shown in the overview.
type SecurityTotals struct {
	Images            int        `json:"images"`
	ScannedImages     int        `json:"scannedImages"`
	Containers        int        `json:"containers"`
	ScannedContainers int        `json:"scannedContainers"`
	Critical          int        `json:"critical"`
	High              int        `json:"high"`
	Medium            int        `json:"medium"`
	Low               int        `json:"low"`
	Unknown           int        `json:"unknown"`
	Total             int        `json:"total"`
	Scanning          int        `json:"scanning"`
	Failed            int        `json:"failed"`
	Stale             int        `json:"stale"`
	LastScanAt        *time.Time `json:"lastScanAt,omitempty"`
}

// SecurityOverview is the merged Trivy view across every container image.
type SecurityOverview struct {
	Enabled bool            `json:"enabled"`
	Images  []SecurityImage `json:"images"`
	Totals  SecurityTotals  `json:"totals"`
}

// TrivyStatus describes the managed Trivy installation and database.
type TrivyStatus struct {
	Enabled      bool       `json:"enabled"`
	Installed    bool       `json:"installed"`
	RuntimeReady bool       `json:"runtimeReady"`
	Version      string     `json:"version"`
	DBReady      bool       `json:"dbReady"`
	JavaDBReady  bool       `json:"javaDbReady"`
	DBUpdatedAt  *time.Time `json:"dbUpdatedAt,omitempty"`
	DBSize       int64      `json:"dbSize"`
	JavaDBSize   int64      `json:"javaDbSize"`
	DBUpdateHour int        `json:"dbUpdateHour"`
	Installing   bool       `json:"installing"`
	UpdatingDB   bool       `json:"updatingDb"`
	Stage        string     `json:"stage,omitempty"`
	Error        string     `json:"error,omitempty"`
}

// TrivyVersion is a release published by the Trivy project.
type TrivyVersion struct {
	Version     string    `json:"version"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"publishedAt"`
	Prerelease  bool      `json:"prerelease"`
}
