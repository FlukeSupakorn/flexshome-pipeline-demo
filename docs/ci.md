# Source CI

The short [workflow caller](../.github/workflows/ci.yml) selects the private
[shared Go workflow](https://github.com/FlukeSupakorn/flexshome-platform/blob/b0034d4f25aca73320d9468fb5d824bda57964dd/.github/workflows/go-ci.yaml)
at immutable platform commit b0034d4f25aca73320d9468fb5d824bda57964dd.
Build programs, confinement assertions, tool/image pins, ko configuration and
their tests belong to that platform repository. This repository contains the
application and its tests; it passes no deployment credentials to shared CI.

Pushes to main, dev, feature/** and release/**, same-repository pull requests to
main/dev, and manual dispatch run shared verification. It checks process and
workspace confinement, tracked Go formatting, tests, vet and a versioned build in
the existing nonroot ARC code pool. The called workflow resolves its own private
composite action through GitHub's native $/ action reference. It uses the caller
source checkout; no shared program is copied into this application.

Only a successful main push publishes the immutable linux/amd64 image. PR and
feature runs verify and skip publication. The shared publisher uses its generated
ko configuration, a digest-pinned base and toolchain, and temporary registry
authentication with cleanup. A lookup tag never replaces the verified image
digest. Source CI cannot write manifests, promote releases or access the cluster.

The seven-day handoff is the uniquely named artifact
release-<source SHA>-<workflow run ID>-<attempt>, containing only release.json:

    {
      "schema": "flexshome.release/v2",
      "source_repository": "FlukeSupakorn/flexshome-pipeline-demo",
      "source_sha": "<full main commit SHA>",
      "workflow_run_id": "<decimal run ID>",
      "workflow_run_attempt": "<decimal attempt>",
      "image_ref": "ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:<digest>",
      "image_digest": "sha256:<digest>"
    }

Manifest/catalog promotion independently verifies that exact repository, source
commit, native workflow run/attempt and image provenance. The shared platform owns
those gates and retained deployment evidence. A source CI change alone does not
change the running deployment or accepted historical release records.

Local application checks: gofmt -l ./cmd, go test -count=1 ./...,
go vet ./... and go build -trimpath ./cmd/server. Caller validation and shared
program tests run from the platform repository. Local checks do not prove private
reusable-action access, ARC execution or main-only publication; native consumer
PR/main runs provide those separate results.
