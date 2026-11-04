package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurora/internal/model"
)

func networkTestProvider(t *testing.T, content string) *Live {
	t.Helper()
	boot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(boot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "config", "network.cfg"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Live{bootDir: boot, stateDir: t.TempDir()}
}

const indexedNetworkConfig = `# Generated settings:
IFNAME[0]="br0"
BONDING[0]="no"
BONDNAME[0]="bond0"
BONDNICS[0]=""
BONDING_MODE[0]="1"
BONDING_MIIMON[0]="100"
BRIDGING[0]="yes"
BRNAME[0]="br0"
BRNICS[0]="eth0"
BRSTP[0]="0"
BRFD[0]="0"
USE_DHCP[0]="no"
IPADDR[0]="192.168.1.10"
NETMASK[0]="255.255.255.0"
GATEWAY[0]="192.168.1.1"
DNS_SERVER1="1.1.1.1"
DNS_SERVER2=""
DNS_SERVER3=""
USE_MTU[0]="no"
MTU[0]="1500"
SYSNICS="1"
`

const multiInterfaceNetworkConfig = `# Generated settings:
IFNAME[0]="br0"
BONDING[0]="no"
BONDNAME[0]="bond0"
BONDNICS[0]=""
BONDING_MODE[0]="1"
BONDING_MIIMON[0]="100"
BRIDGING[0]="yes"
BRNAME[0]="br0"
BRNICS[0]="eth0"
BRSTP[0]="0"
BRFD[0]="0"
USE_DHCP[0]="no"
IPADDR[0]="192.168.1.10"
NETMASK[0]="255.255.255.0"
GATEWAY[0]="192.168.1.1"
USE_MTU[0]="yes"
MTU[0]="9000"
IFNAME[1]="br1"
BONDING[1]="no"
BONDNAME[1]="bond0"
BONDNICS[1]=""
BONDING_MODE[1]="1"
BONDING_MIIMON[1]="100"
BRIDGING[1]="yes"
BRNAME[1]="br1"
BRNICS[1]="eth1"
BRSTP[1]="0"
BRFD[1]="0"
USE_DHCP[1]="yes"
IPADDR[1]=""
NETMASK[1]=""
GATEWAY[1]=""
USE_MTU[1]="no"
MTU[1]="1400"
DNS_SERVER1="1.1.1.1"
DNS_SERVER2=""
DNS_SERVER3=""
SYSNICS="2"
`

func TestNetworkConfigReadsIndexedFormat(t *testing.T) {
	live := networkTestProvider(t, indexedNetworkConfig)
	cfg, err := live.NetworkConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Format != "indexed" || len(cfg.Interfaces) != 1 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	iface := cfg.Interfaces[0]
	if iface.Interface != "br0" || iface.IPAddr != "192.168.1.10" || iface.UseDHCP {
		t.Fatalf("unexpected interface: %+v", iface)
	}
	if !iface.Bridging || iface.BridgeName != "br0" || iface.BridgeNICs[0] != "eth0" {
		t.Fatalf("unexpected bridge: %+v", iface)
	}
	if len(cfg.DNS) != 1 || cfg.DNS[0] != "1.1.1.1" {
		t.Fatalf("unexpected DNS: %+v", cfg.DNS)
	}
}

func TestNetworkConfigReadsEveryInterface(t *testing.T) {
	live := networkTestProvider(t, multiInterfaceNetworkConfig)
	cfg, err := live.NetworkConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Interfaces) != 2 {
		t.Fatalf("expected two interfaces, got %+v", cfg.Interfaces)
	}
	if cfg.Interfaces[0].Interface != "br0" || cfg.Interfaces[1].Interface != "br1" {
		t.Fatalf("unexpected interface order: %+v", cfg.Interfaces)
	}
	if !cfg.Interfaces[1].UseDHCP || cfg.Interfaces[1].BridgeNICs[0] != "eth1" {
		t.Fatalf("unexpected second interface: %+v", cfg.Interfaces[1])
	}
	if !cfg.Interfaces[0].UseMTU || cfg.Interfaces[0].MTU != 9000 {
		t.Fatalf("unexpected first MTU: %+v", cfg.Interfaces[0])
	}
	if cfg.Interfaces[1].UseMTU || cfg.Interfaces[1].MTU != 1400 {
		t.Fatalf("unexpected second MTU: %+v", cfg.Interfaces[1])
	}
}

func TestSaveNetworkConfigKeepsOtherInterfaces(t *testing.T) {
	live := networkTestProvider(t, multiInterfaceNetworkConfig)
	cfg, err := live.NetworkConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Interfaces[1].UseDHCP = false
	cfg.Interfaces[1].IPAddr = "10.0.0.20"
	cfg.Interfaces[1].Netmask = "255.255.255.0"
	cfg.Interfaces[1].Gateway = "10.0.0.1"
	cfg.DNS = []string{"9.9.9.9", "1.1.1.1"}
	saved, err := live.SaveNetworkConfig(context.Background(), cfg, "192.168.1.50", false)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Backup == "" {
		t.Fatal("expected a backup file name")
	}
	raw, err := os.ReadFile(filepath.Join(live.bootDir, "config", "network.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{
		`IFNAME[1]="br1"`,
		`IPADDR[1]="10.0.0.20"`,
		`GATEWAY[1]="10.0.0.1"`,
		`USE_DHCP[1]="no"`,
		`BRNICS[1]="eth1"`,
		`IPADDR[0]="192.168.1.10"`,
		`DNS_SERVER1="9.9.9.9"`,
		`SYSNICS="2"`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %s in %s", expected, text)
		}
	}
	backups, _ := filepath.Glob(filepath.Join(live.bootDir, "config", "network.cfg.aurora-*.bak"))
	if len(backups) != 1 {
		t.Fatalf("expected one backup, got %d", len(backups))
	}
}

func TestSaveNetworkConfigRejectsLockout(t *testing.T) {
	live := networkTestProvider(t, indexedNetworkConfig)
	cfg, _ := live.NetworkConfig(context.Background())
	cfg.Interfaces[0].IPAddr = "10.0.0.20"
	cfg.Interfaces[0].Gateway = "10.0.0.1"
	cfg.Interfaces[0].Netmask = "255.255.255.0"
	if _, err := live.SaveNetworkConfig(context.Background(), cfg, "192.168.1.50", false); !errors.Is(err, ErrNetworkLockout) {
		t.Fatalf("expected lockout error, got %v", err)
	}
	if _, err := live.SaveNetworkConfig(context.Background(), cfg, "192.168.1.50", true); err != nil {
		t.Fatalf("explicit acknowledgement should allow the change: %v", err)
	}
}

func TestSecondaryInterfaceDoesNotFlagUnrelatedClient(t *testing.T) {
	live := networkTestProvider(t, multiInterfaceNetworkConfig)
	cfg, _ := live.NetworkConfig(context.Background())
	cfg.Interfaces[1].UseDHCP = false
	cfg.Interfaces[1].IPAddr = "10.0.0.20"
	cfg.Interfaces[1].Netmask = "255.255.255.0"
	cfg.Interfaces[1].Gateway = "10.0.0.1"
	if _, err := live.SaveNetworkConfig(context.Background(), cfg, "192.168.1.50", false); err != nil {
		t.Fatalf("a client on another subnet should not block a secondary interface: %v", err)
	}
}

func TestSaveNetworkConfigValidatesFields(t *testing.T) {
	live := networkTestProvider(t, indexedNetworkConfig)
	cfg, _ := live.NetworkConfig(context.Background())
	cfg.Interfaces[0].Netmask = "255.0.255.0"
	if _, err := live.SaveNetworkConfig(context.Background(), cfg, "", false); err == nil {
		t.Fatal("expected invalid subnet mask to be rejected")
	}
	cfg, _ = live.NetworkConfig(context.Background())
	cfg.Interfaces[0].Gateway = "10.0.0.1"
	if _, err := live.SaveNetworkConfig(context.Background(), cfg, "", false); err == nil {
		t.Fatal("expected out-of-subnet gateway to be rejected")
	}
}

func TestRevertNetworkConfig(t *testing.T) {
	live := networkTestProvider(t, indexedNetworkConfig)
	cfg, _ := live.NetworkConfig(context.Background())
	cfg.Interfaces[0].IPAddr = "192.168.1.99"
	if _, err := live.SaveNetworkConfig(context.Background(), cfg, "192.168.1.50", false); err != nil {
		t.Fatal(err)
	}
	backup, err := live.RevertNetworkConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if backup == "" {
		t.Fatal("expected backup name")
	}
	restored, _ := live.NetworkConfig(context.Background())
	if restored.Interfaces[0].IPAddr != "192.168.1.10" {
		t.Fatalf("expected original address, got %s", restored.Interfaces[0].IPAddr)
	}
}

func TestNetworkInterfaceLockoutRisk(t *testing.T) {
	old := model.NetworkInterfaceConfig{Index: 0, IPAddr: "192.168.1.10", Netmask: "255.255.255.0"}
	updated := model.NetworkInterfaceConfig{Index: 0, IPAddr: "192.168.1.10", Netmask: "255.255.255.0"}
	if networkInterfaceLockoutRisk(old, updated, "192.168.1.20") {
		t.Fatal("client inside the subnet must not be flagged")
	}
	updated.IPAddr = "10.0.0.10"
	if !networkInterfaceLockoutRisk(old, updated, "192.168.1.20") {
		t.Fatal("client outside the new subnet must be flagged")
	}
	secondaryOld := model.NetworkInterfaceConfig{Index: 1, IPAddr: "10.0.0.10", Netmask: "255.255.255.0"}
	secondaryNew := model.NetworkInterfaceConfig{Index: 1, IPAddr: "10.0.1.10", Netmask: "255.255.255.0"}
	if networkInterfaceLockoutRisk(secondaryOld, secondaryNew, "192.168.1.20") {
		t.Fatal("a client on another subnet must not be flagged for a secondary interface")
	}
}

func TestSaveNetworkConfigDisablingBridgeRestoresPhysicalName(t *testing.T) {
	live := networkTestProvider(t, indexedNetworkConfig)
	cfg, _ := live.NetworkConfig(context.Background())
	if cfg.Interfaces[0].PhysicalNIC != "eth0" {
		t.Fatalf("expected eth0 physical NIC, got %q", cfg.Interfaces[0].PhysicalNIC)
	}
	cfg.Interfaces[0].Bridging = false
	saved, err := live.SaveNetworkConfig(context.Background(), cfg, "192.168.1.50", false)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Interfaces[0].Interface != "eth0" {
		t.Fatalf("expected the returned interface to be eth0, got %q", saved.Interfaces[0].Interface)
	}
	raw, err := os.ReadFile(filepath.Join(live.bootDir, "config", "network.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `IFNAME[0]="eth0"`) || !strings.Contains(string(raw), `BRIDGING[0]="no"`) {
		t.Fatalf("unexpected saved config: %s", raw)
	}
}

func TestNetworkConfigReadsCheckboxMTU(t *testing.T) {
	live := networkTestProvider(t, `IFNAME[0]="br0"
BRIDGING[0]="no"
USE_DHCP[0]="yes"
USE_MTU[0]="on"
MTU[0]="9000"
SYSNICS="1"
`)
	cfg, err := live.NetworkConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Interfaces[0].UseMTU || cfg.Interfaces[0].MTU != 9000 {
		t.Fatalf("unexpected MTU: %+v", cfg.Interfaces[0])
	}
}

func TestParseYesAcceptsCheckboxValues(t *testing.T) {
	for _, value := range []string{"yes", "on", "true", "1", " YES "} {
		if !parseYes(value) {
			t.Fatalf("expected %q to be truthy", value)
		}
	}
	for _, value := range []string{"", "no", "off", "false", "0"} {
		if parseYes(value) {
			t.Fatalf("expected %q to be falsy", value)
		}
	}
}

func TestNetworkInterfaceStateUnknownForAbsentInterface(t *testing.T) {
	live := networkTestProvider(t, `IFNAME[0]="brtestaurora0"
BRIDGING[0]="no"
BONDING[0]="no"
USE_DHCP[0]="yes"
SYSNICS="1"
`)
	cfg, err := live.NetworkConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interfaces[0].State != "unknown" {
		t.Fatalf("expected unknown state, got %q", cfg.Interfaces[0].State)
	}
}

func TestSetNetworkInterfaceStateValidatesInput(t *testing.T) {
	live := networkTestProvider(t, indexedNetworkConfig)
	if err := live.SetNetworkInterfaceState(context.Background(), "../eth0", true); err == nil {
		t.Fatal("invalid interface name was accepted")
	}
	if err := live.SetNetworkInterfaceState(context.Background(), "eth99", true); err == nil {
		t.Fatal("unconfigured interface was accepted")
	}
	if err := live.SetNetworkInterfaceState(context.Background(), "br0", true); err == nil {
		t.Fatal("absent interface was accepted")
	}
}
