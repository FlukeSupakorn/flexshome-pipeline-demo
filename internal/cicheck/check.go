package main

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
)

// checkConfinement fails closed when the job cannot prove the expected kernel
// security state. The controller-side hook enforces these controls before any
// repository code runs; this check supplies an independent in-job assertion.
func checkConfinement(files fs.FS, uid, gid int, environ []string) error {
	if uid != 1001 || gid != 1001 {
		return fmt.Errorf("job must run as UID/GID 1001:1001")
	}
	status, err := fs.ReadFile(files, "proc/self/status")
	if err != nil {
		return fmt.Errorf("cannot inspect process security status: %w", err)
	}
	fields := make(map[string][]string)
	for line := range strings.SplitSeq(string(status), "\n") {
		if key, value, ok := strings.Cut(line, ":"); ok {
			fields[key] = strings.Fields(value)
		}
	}
	for _, key := range []string{"Uid", "Gid"} {
		values := fields[key]
		if len(values) != 4 {
			return fmt.Errorf("missing or malformed process %s", key)
		}
		for _, value := range values {
			if value != "1001" {
				return fmt.Errorf("unsafe process %s", key)
			}
		}
	}
	for _, key := range []string{"CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"} {
		values := fields[key]
		if len(values) != 1 {
			return fmt.Errorf("missing or malformed process %s", key)
		}
		value, err := strconv.ParseUint(values[0], 16, 64)
		if err != nil || value != 0 {
			return fmt.Errorf("job has unsafe process %s", key)
		}
	}
	for key, want := range map[string]string{"NoNewPrivs": "1", "Seccomp": "2"} {
		if values := fields[key]; len(values) != 1 || values[0] != want {
			return fmt.Errorf("job requires %s=%s", key, want)
		}
	}
	for _, name := range []string{
		"var/run/secrets/kubernetes.io/serviceaccount/token",
		"run/secrets/kubernetes.io/serviceaccount/token",
		"var/run/docker.sock", "run/docker.sock",
		"run/containerd/containerd.sock", "run/k3s/containerd/containerd.sock",
		"var/run/crio/crio.sock", "run/podman/podman.sock",
	} {
		if _, err := fs.Stat(files, name); err == nil {
			return fmt.Errorf("forbidden credential/runtime mount: /%s", name)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("cannot rule out credential/runtime mount: /%s", name)
		}
	}
	for _, entry := range environ {
		name, value, _ := strings.Cut(entry, "=")
		switch name {
		case "KUBECONFIG", "DOCKER_HOST", "CONTAINER_HOST", "GITHUB_TOKEN", "GH_TOKEN",
			"AWS_WEB_IDENTITY_TOKEN_FILE", "AZURE_FEDERATED_TOKEN_FILE", "GOOGLE_APPLICATION_CREDENTIALS":
			if value != "" {
				// Never put environment values into errors or evidence artifacts.
				return fmt.Errorf("forbidden ambient credential/runtime environment: %s", name)
			}
		}
	}
	return nil
}

type release struct {
	Schema           string `json:"schema"`
	SourceRepository string `json:"source_repository"`
	SourceSHA        string `json:"source_sha"`
	WorkflowRunID    string `json:"workflow_run_id"`
	ImageRef         string `json:"image_ref"`
	ImageDigest      string `json:"image_digest"`
}

var sourceSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
var imageRef = regexp.MustCompile(`^(ghcr\.io/flukesupakorn/flexshome-pipeline-demo)(?::sha-([0-9a-f]{40}))?@(sha256:[0-9a-f]{64})$`)
var runID = regexp.MustCompile(`^[1-9][0-9]*$`)

func newRelease(sha, run, refs string) (release, error) {
	if !sourceSHA.MatchString(sha) || !runID.MatchString(run) {
		return release{}, fmt.Errorf("release requires a full source SHA and positive workflow run ID")
	}
	ref := strings.TrimSpace(refs)
	match := imageRef.FindStringSubmatch(ref)
	if match == nil || (match[2] != "" && match[2] != sha) {
		return release{}, fmt.Errorf("release requires exactly one published digest in the demo image repository")
	}
	// ko retains the requested source tag in --image-refs output. Bind that tag
	// to this source SHA, then publish only the immutable repository@digest.
	return release{
		Schema: "flexshome.release/v1", SourceRepository: "FlukeSupakorn/flexshome-pipeline-demo",
		SourceSHA: sha, WorkflowRunID: run, ImageRef: match[1] + "@" + match[3], ImageDigest: match[3],
	}, nil
}
