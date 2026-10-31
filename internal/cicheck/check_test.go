package main

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

const confinedStatus = `Name:	go
Uid:	1001	1001	1001	1001
Gid:	1001	1001	1001	1001
CapInh:	0000000000000000
CapPrm:	0000000000000000
CapEff:	0000000000000000
CapBnd:	0000000000000000
CapAmb:	0000000000000000
NoNewPrivs:	1
Seccomp:	2
`

func confinedFS() fstest.MapFS {
	return fstest.MapFS{"proc/self/status": &fstest.MapFile{Data: []byte(confinedStatus)}}
}

func TestConfinementAcceptsUnprivilegedJob(t *testing.T) {
	if err := checkConfinement(confinedFS(), 1001, 1001, nil); err != nil {
		t.Fatal(err)
	}
}

func TestConfinementRejectsUnsafeIdentity(t *testing.T) {
	for _, ids := range [][2]int{{0, 0}, {1001, 0}, {1000, 1001}} {
		if err := checkConfinement(confinedFS(), ids[0], ids[1], nil); err == nil {
			t.Fatalf("accepted UID/GID %v", ids)
		}
	}
}

func TestConfinementRejectsMissingOrUnsafeKernelState(t *testing.T) {
	for _, tc := range []struct{ field, from, to string }{
		{"inheritable capability", "CapInh:\t0000000000000000", "CapInh:\t0000000000000001"},
		{"permitted capability", "CapPrm:\t0000000000000000", "CapPrm:\t0000000000000001"},
		{"effective capability", "CapEff:\t0000000000000000", "CapEff:\t0000000000000001"},
		{"bounding capability", "CapBnd:\t0000000000000000", "CapBnd:\t0000000000000001"},
		{"ambient capability", "CapAmb:\t0000000000000000", "CapAmb:\t0000000000000001"},
		{"privilege escalation", "NoNewPrivs:\t1", "NoNewPrivs:\t0"},
		{"unconfined seccomp", "Seccomp:\t2", "Seccomp:\t0"},
		{"missing capability", "CapEff:\t0000000000000000\n", ""},
		{"missing seccomp", "Seccomp:\t2\n", ""},
		{"effective root", "Uid:\t1001\t1001\t1001\t1001", "Uid:\t1001\t0\t1001\t1001"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			files := confinedFS()
			files["proc/self/status"].Data = []byte(strings.Replace(confinedStatus, tc.from, tc.to, 1))
			if err := checkConfinement(files, 1001, 1001, nil); err == nil {
				t.Fatal("unsafe kernel state accepted")
			}
		})
	}
	if err := checkConfinement(fstest.MapFS{}, 1001, 1001, nil); err == nil {
		t.Fatal("missing proc status accepted")
	}
}

func TestConfinementRejectsCredentialAndRuntimeMounts(t *testing.T) {
	for _, name := range []string{
		"var/run/secrets/kubernetes.io/serviceaccount/token",
		"run/secrets/kubernetes.io/serviceaccount/token",
		"var/run/docker.sock", "run/docker.sock",
		"run/containerd/containerd.sock", "run/k3s/containerd/containerd.sock",
		"var/run/crio/crio.sock", "run/podman/podman.sock",
	} {
		t.Run(name, func(t *testing.T) {
			files := confinedFS()
			files[name] = &fstest.MapFile{Mode: fs.ModeSocket}
			if err := checkConfinement(files, 1001, 1001, nil); err == nil {
				t.Fatal("credential/runtime mount accepted")
			}
		})
	}
}

func TestConfinementRejectsAmbientCredentialsWithoutEchoingValues(t *testing.T) {
	for _, name := range []string{"KUBECONFIG", "DOCKER_HOST", "CONTAINER_HOST", "GITHUB_TOKEN", "GH_TOKEN", "AWS_WEB_IDENTITY_TOKEN_FILE", "AZURE_FEDERATED_TOKEN_FILE", "GOOGLE_APPLICATION_CREDENTIALS"} {
		t.Run(name, func(t *testing.T) {
			err := checkConfinement(confinedFS(), 1001, 1001, []string{name + "=sensitive-value"})
			if err == nil || strings.Contains(err.Error(), "sensitive-value") {
				t.Fatalf("must reject without disclosing value; got %v", err)
			}
		})
	}
}

func TestReleaseUsesExactlyThePublishedDigest(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	const digest = "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	const image = "ghcr.io/flukesupakorn/flexshome-pipeline-demo@" + digest
	release, err := newRelease(sha, "12345", image+"\n")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"flexshome.release/v1","source_repository":"FlukeSupakorn/flexshome-pipeline-demo","source_sha":"0123456789abcdef0123456789abcdef01234567","workflow_run_id":"12345","image_ref":"ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","image_digest":"sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}`
	if string(data) != want {
		t.Fatalf("release = %s", data)
	}
}

func TestReleaseRejectsTagsMultipleImagesAndMalformedInputs(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	image := "ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct{ name, sha, runID, refs string }{
		{"tag", sha, "1", "ghcr.io/flukesupakorn/flexshome-pipeline-demo:latest"},
		{"multiple images", sha, "1", image + "\n" + image},
		{"wrong repository", sha, "1", strings.Replace(image, "pipeline-demo", "other", 1)},
		{"short digest", sha, "1", image[:len(image)-1]},
		{"nonhex digest", sha, "1", image[:len(image)-1] + "z"},
		{"missing source", "", "1", image},
		{"short source", sha[:7], "1", image},
		{"nonhex source", "z" + sha[1:], "1", image},
		{"missing run", sha, "", image},
		{"zero run", sha, "0", image},
		{"negative run", sha, "-1", image},
		{"nonnumeric run", sha, "1/x", image},
		{"missing reference", sha, "1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := newRelease(tc.sha, tc.runID, tc.refs); err == nil {
				t.Fatal("invalid release accepted")
			}
		})
	}
}
