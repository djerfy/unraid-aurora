package model

// VMCloneOptions creates a new domain from an existing definition.
type VMCloneOptions struct {
	Name  string `json:"name"`
	Start bool   `json:"start"`
}

type VMDiskResizeOptions struct {
	SizeGB int64 `json:"sizeGB"`
}

// VMNetworkConfig is the editable subset of a libvirt interface.
type VMNetworkConfig struct {
	Bridge string `json:"bridge"`
	Model  string `json:"model"`
	MAC    string `json:"mac"`
}

// VMHostDeviceRef points at a PCI address or a USB vendor/product pair.
type VMHostDeviceRef struct {
	Type    string `json:"type"`
	Address string `json:"address,omitempty"`
	Vendor  string `json:"vendor,omitempty"`
	Product string `json:"product,omitempty"`
}

// VMAdvancedConfig is the guided hardware editor. Unknown XML elements are
// preserved; only these fields are rewritten.
type VMAdvancedConfig struct {
	VCPUs       int               `json:"vcpus"`
	MemoryMB    int               `json:"memoryMB"`
	CPUSet      string            `json:"cpuSet"`
	Networks    []VMNetworkConfig `json:"networks"`
	SoundModel  string            `json:"soundModel"`
	HostDevices []VMHostDeviceRef `json:"hostDevices"`
}

// VMHostDevice is an assignable PCI or USB device discovered on the host.
type VMHostDevice struct {
	Type       string `json:"type"`
	Address    string `json:"address,omitempty"`
	Vendor     string `json:"vendor,omitempty"`
	Product    string `json:"product,omitempty"`
	Name       string `json:"name"`
	Driver     string `json:"driver,omitempty"`
	IOMMUGroup string `json:"iommuGroup,omitempty"`
	Class      string `json:"class,omitempty"`
}
