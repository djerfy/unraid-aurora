package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurora/internal/model"
)

func TestParseImageRef(t *testing.T) {
	cases := []struct {
		image  string
		server string
		repo   string
		tag    string
		pull   string
	}{
		{"nginx", "index.docker.io", "library/nginx", "latest", "nginx"},
		{"nginx:1.27", "index.docker.io", "library/nginx", "1.27", "nginx"},
		{"plexinc/pms-docker:latest", "index.docker.io", "plexinc/pms-docker", "latest", "plexinc/pms-docker"},
		{"ghcr.io/foo/bar:edge", "ghcr.io", "foo/bar", "edge", "ghcr.io/foo/bar"},
		{"localhost:5000/app:v2", "localhost:5000", "app", "v2", "localhost:5000/app"},
		{"lscr.io/linuxserver/plex", "lscr.io", "linuxserver/plex", "latest", "lscr.io/linuxserver/plex"},
	}
	for _, tc := range cases {
		ref, err := parseImageRef(tc.image)
		if err != nil {
			t.Fatalf("%s: %v", tc.image, err)
		}
		if ref.server != tc.server || ref.repository != tc.repo || ref.tag != tc.tag || ref.pullName() != tc.pull {
			t.Fatalf("%s: unexpected ref %+v", tc.image, ref)
		}
	}
	if _, err := parseImageRef("bad image"); err == nil {
		t.Fatal("whitespace should be rejected")
	}
	if ref, err := parseImageRef("nginx@sha256:abc"); err != nil || ref.digest != "sha256:abc" {
		t.Fatalf("digest ref: %+v %v", ref, err)
	}
}

func TestFetchRemoteDigestWithBearerToken(t *testing.T) {
	const digest = "sha256:1234567890abcdef"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			if r.URL.Query().Get("scope") != "repository:library/nginx:pull" {
				t.Errorf("unexpected scope %q", r.URL.Query().Get("scope"))
			}
			fmt.Fprint(w, `{"token":"test-token"}`)
		case strings.HasPrefix(r.URL.Path, "/v2/library/nginx/manifests/"):
			if r.Header.Get("Authorization") != "Bearer test-token" {
				w.Header().Set("Www-Authenticate",
					fmt.Sprintf(`Bearer realm="%s/token",service="registry",scope="repository:library/nginx:pull"`, server.URL))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if !strings.Contains(r.Header.Get("Accept"), "application/vnd.oci.image.index.v1+json") {
				t.Errorf("missing manifest accept header: %q", r.Header.Get("Accept"))
			}
			w.Header().Set("Docker-Content-Digest", digest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ref := imageRef{apiBase: server.URL, server: "test", repository: "library/nginx", tag: "latest"}
	got, err := fetchRemoteDigest(context.Background(), server.Client(), ref, dockerCredentials{})
	if err != nil {
		t.Fatalf("fetch digest: %v", err)
	}
	if got != digest {
		t.Fatalf("digest = %q, want %q", got, digest)
	}
}

func TestBuildContainerCreateBody(t *testing.T) {
	body := buildContainerCreateBody(model.ContainerCreateOptions{
		Name:          "plex",
		Image:         "linuxserver/plex:latest",
		NetworkMode:   "br0",
		RestartPolicy: "unless-stopped",
		Ports:         []model.Port{{Host: 32400, Container: 32400, Protocol: "tcp"}},
		Volumes:       []model.ContainerVolume{{Source: "/mnt/user/appdata/plex", Destination: "/config"}},
		Env:           []model.ContainerEnv{{Key: "PUID", Value: "99"}},
		Icon:          "https://example.test/plex.png",
		WebUI:         "http://[IP]:[PORT:32400]/",
		Privileged:    true,
	})
	if body["Image"] != "linuxserver/plex:latest" {
		t.Fatalf("image = %v", body["Image"])
	}
	host, _ := body["HostConfig"].(map[string]any)
	if host == nil || host["NetworkMode"] != "br0" || host["Privileged"] != true {
		t.Fatalf("host config = %+v", host)
	}
	restart, _ := host["RestartPolicy"].(map[string]any)
	if restart["Name"] != "unless-stopped" {
		t.Fatalf("restart policy = %+v", restart)
	}
	binds, _ := host["Binds"].([]any)
	if len(binds) != 1 || binds[0] != "/mnt/user/appdata/plex:/config" {
		t.Fatalf("binds = %+v", binds)
	}
	labels, _ := body["Labels"].(map[string]string)
	if labels["net.unraid.docker.icon"] == "" || labels["net.unraid.docker.webui"] == "" {
		t.Fatalf("labels = %+v", labels)
	}
}

func TestValidateContainerCreateOptions(t *testing.T) {
	valid := model.ContainerCreateOptions{
		Name:          "plex",
		Image:         "linuxserver/plex:latest",
		RestartPolicy: "unless-stopped",
		Ports:         []model.Port{{Host: 32400, Container: 32400, Protocol: "tcp"}},
	}
	if err := ValidateContainerCreateOptions(valid); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
	for name, mutate := range map[string]func(*model.ContainerCreateOptions){
		"bad name":      func(o *model.ContainerCreateOptions) { o.Name = "bad name" },
		"missing image": func(o *model.ContainerCreateOptions) { o.Image = "" },
		"bad network":   func(o *model.ContainerCreateOptions) { o.NetworkMode = "a b" },
		"bad policy":    func(o *model.ContainerCreateOptions) { o.RestartPolicy = "sometimes" },
		"bad port": func(o *model.ContainerCreateOptions) {
			o.Ports = []model.Port{{Host: 0, Container: 32400, Protocol: "tcp"}}
		},
	} {
		options := valid
		mutate(&options)
		if err := ValidateContainerCreateOptions(options); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
}
