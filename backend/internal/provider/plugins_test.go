package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const samplePlugin = `<!DOCTYPE PLUGIN [
<!ENTITY name    "my-plugin">
<!ENTITY version "2024.02.01">
]>
<PLUGIN name="&name;" author="Jane Doe" version="&version;" pluginURL="https://example.test/my-plugin.plg" category="Tools">
<DESCRIPTION><![CDATA[Adds <b>useful</b> tooling to Unraid.]]></DESCRIPTION>
</PLUGIN>
`

func TestParsePluginFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "my-plugin.plg")
	if err := os.WriteFile(path, []byte(samplePlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	plugin, err := parsePluginFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plugin.Name != "my-plugin" || plugin.Version != "2024.02.01" {
		t.Fatalf("identity = %+v", plugin)
	}
	if plugin.Author != "Jane Doe" || plugin.Category != "Tools" {
		t.Fatalf("metadata = %+v", plugin)
	}
	if plugin.PluginURL != "https://example.test/my-plugin.plg" {
		t.Fatalf("url = %q", plugin.PluginURL)
	}
	if !strings.Contains(plugin.Description, "useful") || strings.Contains(plugin.Description, "<b>") {
		t.Fatalf("description = %q", plugin.Description)
	}
}

func TestPluginListReadsBootDirectory(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(boot, "config", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "my-plugin.plg"), []byte(samplePlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	plugins := live.pluginList()
	if len(plugins) != 1 || plugins[0].Name != "my-plugin" || plugins[0].Filename != "my-plugin.plg" {
		t.Fatalf("plugins = %+v", plugins)
	}
}

func TestVersionNewer(t *testing.T) {
	if !versionNewer("2024.02.01", "2024.01.31") {
		t.Fatal("newer date should be detected")
	}
	if versionNewer("2024.01.31", "2024.02.01") {
		t.Fatal("older version should not win")
	}
	if versionNewer("2024.01.31", "2024.01.31") {
		t.Fatal("equal versions should not update")
	}
	if !versionNewer("1.2.0-beta2", "1.2.0-beta1") {
		t.Fatal("suffix increment should be detected")
	}
}
