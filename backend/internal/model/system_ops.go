package model

import "time"

// OSStatus describes the installed Unraid release and the last update check.
type OSStatus struct {
	Version           string     `json:"version"`
	Branch            string     `json:"branch"`
	Latest            string     `json:"latest,omitempty"`
	LatestName        string     `json:"latestName,omitempty"`
	UpdateAvailable   bool       `json:"updateAvailable"`
	Eligible          bool       `json:"eligible"`
	Changelog         string     `json:"changelog,omitempty"`
	ChangelogPretty   string     `json:"changelogPretty,omitempty"`
	ReleaseDate       string     `json:"releaseDate,omitempty"`
	CheckedAt         *time.Time `json:"checkedAt,omitempty"`
	CheckError        string     `json:"checkError,omitempty"`
	RebootRequired    bool       `json:"rebootRequired"`
	RebootType        string     `json:"rebootType,omitempty"`
	RebootVersion     string     `json:"rebootVersion,omitempty"`
	PreviousVersion   string     `json:"previousVersion,omitempty"`
	PreviousDate      string     `json:"previousDate,omitempty"`
	RestoreAvailable  bool       `json:"restoreAvailable"`
	ThirdPartyDrivers bool       `json:"thirdPartyDrivers"`
}

// UnraidAPIStatus mirrors the health of the official unraid-api service so the
// UI can show the same component the Unraid WebGUI exposes.
type UnraidAPIStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	State     string `json:"state"`
	Version   string `json:"version"`
	Raw       string `json:"raw"`
	Config    string `json:"config,omitempty"`
	LogLevel  string `json:"logLevel,omitempty"`
}

// NetworkInterfaceConfig is the editable subset of one interface block in
// /boot/config/network.cfg. Values are read from either the legacy flat format
// or the indexed Unraid 6.12+ format.
type NetworkInterfaceConfig struct {
	Index       int      `json:"index"`
	Interface   string   `json:"interface"`
	PhysicalNIC string   `json:"physicalNic"`
	State       string   `json:"state"`
	UseDHCP     bool     `json:"useDhcp"`
	IPAddr      string   `json:"ipAddr"`
	Netmask     string   `json:"netmask"`
	Gateway     string   `json:"gateway"`
	UseMTU      bool     `json:"useMtu"`
	MTU         int      `json:"mtu"`
	Bonding     bool     `json:"bonding"`
	BondName    string   `json:"bondName"`
	BondNICs    []string `json:"bondNics"`
	BondMode    string   `json:"bondMode"`
	BondMIIMon  int      `json:"bondMiiMon"`
	Bridging    bool     `json:"bridging"`
	BridgeName  string   `json:"bridgeName"`
	BridgeNICs  []string `json:"bridgeNics"`
	BridgeSTP   bool     `json:"bridgeStp"`
	BridgeFD    int      `json:"bridgeFd"`
	LockoutRisk bool     `json:"lockoutRisk"`
}

// NetworkConfig exposes every interface stored on the flash drive. DNS is
// global in network.cfg and therefore lives at the top level.
type NetworkConfig struct {
	Interfaces    []NetworkInterfaceConfig `json:"interfaces"`
	DNS           []string                 `json:"dns"`
	AvailableNICs []string                 `json:"availableNics"`
	Format        string                   `json:"format"`
	Backup        string                   `json:"backup,omitempty"`
}

// NetworkConfigUpdate adds the explicit confirmation required when a change
// could make the current client unreachable.
type NetworkConfigUpdate struct {
	NetworkConfig
	Acknowledge bool `json:"acknowledge"`
}
