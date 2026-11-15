# Source CI

The [workflow](../.github/workflows/ci.yml) verifies source on free
GitHub-hosted Linux runners for this public repository. It checks Go formatting,
tests, vet and a compiled binary using the Go version in go.mod. Pull requests
from this repository and branch pushes can verify without package-write access.

Only a successful push to this repository's main branch publishes an immutable
linux/amd64 image. The publication job checks out the same verified commit, uses
checksum-pinned ko and a digest-pinned base image, and scopes registry
authentication to the job's temporary directory. The image records its full
source revision and publication run/attempt as OCI labels.

The seven-day handoff is the uniquely named artifact
`release-<source SHA>-<workflow run ID>-<attempt>`, containing only release.json.

```json
{
  "schema": "flexshome.release/v2",
  "source_repository": "FlukeSupakorn/flexshome-pipeline-demo",
  "source_sha": "<full main commit SHA>",
  "workflow_run_id": "<decimal run ID>",
  "workflow_run_attempt": "<decimal attempt>",
  "image_ref": "ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:<digest>",
  "image_digest": "sha256:<digest>"
}
```

Manifest promotion uses the exact source commit, workflow run and attempt, and
image digest. A successful source run supplies release provenance for the
separate manifest change. See the [release steps](../README.md#release-a-change).
