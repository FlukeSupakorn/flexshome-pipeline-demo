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

func TestReleaseNormalizesKoTaggedDigestForExactSource(t *testing.T) {
	const sha = "1b33a83fc9b0b243adc0aed5aeee1d8e7f57e71e"
	const digest = "sha256:94ec0a8c1c741ded0303d92d2b684273ce12b81914bb30971ea7f28a07e31771"
	const repository = "ghcr.io/flukesupakorn/flexshome-pipeline-demo"
	// ko --tags=sha-$GITHUB_SHA --image-refs retains the source tag alongside
	// the digest. The handoff must retain the digest and remove the tag.
	got, err := newRelease(sha, "41223276915", repository+":sha-"+sha+"@"+digest+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.ImageRef != repository+"@"+digest || got.ImageDigest != digest || got.SourceSHA != sha {
		t.Fatalf("release bindings were not normalized: %+v", got)
	}
}

func TestReleaseRejectsTagsMultipleImagesAndMalformedInputs(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	image := "ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:" + strings.Repeat("a", 64)
	tagged := strings.Replace(image, "@", ":sha-"+sha+"@", 1)
	for _, tc := range []struct{ name, sha, runID, refs string }{
		{"tag", sha, "1", "ghcr.io/flukesupakorn/flexshome-pipeline-demo:latest"},
		{"source tag without digest", sha, "1", "ghcr.io/flukesupakorn/flexshome-pipeline-demo:sha-" + sha},
		{"arbitrary tag with digest", sha, "1", strings.Replace(tagged, "sha-"+sha, "latest", 1)},
		{"wrong source tag", sha, "1", strings.Replace(tagged, "sha-"+sha, "sha-"+strings.Repeat("f", 40), 1)},
		{"short source tag", sha, "1", strings.Replace(tagged, "sha-"+sha, "sha-"+sha[:7], 1)},
		{"uppercase source tag", sha, "1", strings.Replace(tagged, "sha-"+sha, "sha-"+strings.ToUpper(sha), 1)},
		{"wrong tagged repository", sha, "1", strings.Replace(tagged, "pipeline-demo", "other", 1)},
		{"multiple tagged images", sha, "1", tagged + "\n" + tagged},
		{"mixed tagged and digest-only images", sha, "1", tagged + "\n" + image},
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
