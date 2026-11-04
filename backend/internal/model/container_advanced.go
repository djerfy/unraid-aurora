package model

import "encoding/json"

// MarshalJSON keeps empty collections as []/{} so clients can rely on stable
// types even when a zero ContainerAdvanced is serialised.
func (advanced ContainerAdvanced) MarshalJSON() ([]byte, error) {
	type plain ContainerAdvanced
	value := plain(advanced)
	if value.Devices == nil {
		value.Devices = []ContainerDevice{}
	}
	if value.CapAdd == nil {
		value.CapAdd = []string{}
	}
	if value.CapDrop == nil {
		value.CapDrop = []string{}
	}
	if value.Sysctls == nil {
		value.Sysctls = map[string]string{}
	}
	if value.Ulimits == nil {
		value.Ulimits = []ContainerUlimit{}
	}
	if value.Labels == nil {
		value.Labels = map[string]string{}
	}
	if value.ExtraHosts == nil {
		value.ExtraHosts = []string{}
	}
	if value.Tmpfs == nil {
		value.Tmpfs = []string{}
	}
	if value.DNS == nil {
		value.DNS = []string{}
	}
	if value.DNSSearch == nil {
		value.DNSSearch = []string{}
	}
	return json.Marshal(value)
}
