package provider

import "testing"

func TestDigestListContains(t *testing.T) {
	digests := []string{"sha256:aaa", "sha256:bbb"}
	if !digestListContains(digests, "sha256:bbb") {
		t.Fatal("any local repository digest must match")
	}
	if digestListContains(digests, "sha256:ccc") {
		t.Fatal("unknown digest must not match")
	}
	if digestListContains(nil, "sha256:aaa") {
		t.Fatal("empty list must not match")
	}
}
