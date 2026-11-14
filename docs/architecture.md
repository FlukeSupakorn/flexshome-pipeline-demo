# Pipeline demo architecture

This stateless application proves OCI build, private image delivery, deployment health, version promotion and rollback. It contains no family data, databases, credentials or host mounts.

The Cloudflare Tunnel forwards demo traffic to the Kubernetes Service and its nonroot Go HTTP process. Kubernetes probes call `/healthz` and `/readyz`. The release page at `/app/` links to `/api/version`. The existing acceptance check can still call `/version`. Unknown routes and unsupported methods are rejected.

The build injects the source revision as the version. Runtime configuration selects the environment and listen address. Responses reveal only the demo release/environment, not host configuration. Shutdown first fails readiness, then drains HTTP with a bounded deadline.

Build flow: pinned Go toolchain → tests/vet → pinned ko + distroless base → OCI registry digest artifact. No Docker daemon, Dockerfile, privileged job, nested container or Kubernetes credential is needed for this Go adapter. Future applications select their appropriate builder without changing the registry/digest/GitOps release contract.

The [source CI contract](ci.md) documents the ARC confinement assertion, main-only publication, immutable release schema, dependency provenance, acceptance evidence, and recovery boundary.

Deployment definitions live in a separate manifest repo; the central version-control repo selects an immutable manifest revision. This app cannot change cluster infrastructure or deploy itself directly.
