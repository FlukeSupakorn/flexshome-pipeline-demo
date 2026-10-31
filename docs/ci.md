# Source CI and release contract

`Source CI / verify` runs on pushes, same-repository pull requests to `main`, and manual dispatch. It checks confinement, formatting, all Go tests, vet, and a version-injected build. Fork pull requests do not execute on this private self-hosted pool. Use a reviewed branch in this repository for changes.

Both jobs select only the ARC scale set `flexshome-code` and the immutable Go container in the workflow. The job runs as UID/GID `1001:1001`. Go build/module/temp paths and ko's cache are ephemeral directories inside the job workspace. The workflow uses no hosted runner, shared Actions cache, Docker daemon, container build action, privileged container, Kubernetes credentials, or deploy permission.

The centrally maintained ARC hook must enforce nonroot identity, `allowPrivilegeEscalation: false`, all capabilities dropped, runtime-default seccomp, no service-account-token mount, and no runtime sockets or host mounts before repository code runs. The source-side `cicheck preflight` independently asserts the expected process identity, all five capability sets empty, `NoNewPrivs=1`, seccomp filter mode, absent conventional Kubernetes token/runtime socket paths, and absent ambient cluster/cloud/registry credentials. It fails closed if kernel status cannot be read or a forbidden mount cannot be ruled out. Its evidence never includes credential values. This assertion complements the controller hook; source code cannot enforce its own isolation boundary.

Only a successful `push` of `main` in `FlukeSupakorn/flexshome-pipeline-demo` can start `publish`. That job checks out the same source SHA and receives only `contents: read` plus `packages: write`. It installs ko after checking the release archive SHA-256, then uses stdin for the step-scoped `GITHUB_TOKEN` and unsets that variable before building. Registry credentials live only in `.ci-credentials/config.json`, with mode restricted by `umask 077`; a trap and an always-run cleanup remove them. No credentials enter artifacts.

ko builds exactly `./cmd/server`, with `--bare`, one build job, `linux/amd64`, image user `65532:65532`, a digest-pinned distroless base, full source SHA version, commit-date `SOURCE_DATE_EPOCH`, and OCI source/revision labels. The `sha-<source SHA>` tag is a lookup aid. The release handoff always comes from ko's `--image-refs` file and must contain exactly one digest reference in the demo repository.

The only uploaded artifact is `release.json`, retained for seven days:

```json
{
  "schema": "flexshome.release/v1",
  "source_repository": "FlukeSupakorn/flexshome-pipeline-demo",
  "source_sha": "<40 lowercase hexadecimal characters>",
  "workflow_run_id": "<positive decimal ID>",
  "image_ref": "ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:<64 lowercase hexadecimal characters>",
  "image_digest": "sha256:<64 lowercase hexadecimal characters>"
}
```

App manifests consume the digest through their own reviewed change. Keep accepted release records in the platform's durable release ledger before the seven-day Actions artifact expires. This repository cannot promote a deployment or change cluster infrastructure. For recovery, retain the previous accepted image digest and manifest revision; revert a faulty source change and run CI again. Never substitute a rebuilt tag for a previously accepted digest during rollback.

## Dependency provenance

Checked against primary GitHub release/tag APIs on 2026-10-31:

| Dependency | Immutable identity | Primary record |
| --- | --- | --- |
| actions/checkout v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` | [GitHub commit API](https://api.github.com/repos/actions/checkout/commits/v7.0.1) |
| actions/upload-artifact v7.0.2 | `cf430e030ddbb5b0abf93d22962f4752f3646cd9` | [GitHub commit API](https://api.github.com/repos/actions/upload-artifact/commits/v7.0.2) |
| ko v0.19.1 Linux x86_64 archive | SHA-256 `635ac6ea3fd376c935fee597fbb29ab2c2449f49ef1655085fe3aa9c25fed7a5` | [Official release](https://github.com/ko-build/ko/releases/tag/v0.19.1) |

Go and distroless image digests are pinned directly in the workflow and `.ko.yaml`. Action runtime compatibility requires an ARC runner supporting Node 24 actions (runner v2.327.1 or newer). The controller pins and actual ARC execution proof belong to the central platform repository.

## Verification and acceptance

Local verification: `gofmt -l ./cmd ./internal`, `go test -count=1 ./...`, `go vet ./...`, `go build -trimpath ./cmd/server`, and actionlint with `.github/actionlint.yaml`. Unit fixtures cover secure and unsafe process states, mounted tokens/sockets, ambient credentials without value disclosure, and exact immutable release reference validation.

Acceptance still requires a real ARC run after GitHub App installation, an intentional formatting failure on a disposable branch proving `publish` stays skipped, a successful reviewed-main push producing the digest artifact, and independent image/runtime verification. A local test run does not prove the remote security boundary or registry publication.
