# Pipeline demo architecture

This stateless application proves OCI build, private image delivery, deployment health, version promotion and rollback. It contains no family data, databases, credentials or host mounts.

Request flow: public demo Funnel → Kubernetes Service → nonroot Go HTTP process. Kubernetes probes call `/healthz` and `/readyz`; the acceptance check calls `/version`. Root `/` is a small human-readable release page. Unknown routes and unsupported methods are rejected.

The build injects the source revision as the version. Runtime configuration selects the environment and listen address. Responses reveal only the demo release/environment, not host configuration. Shutdown first fails readiness, then drains HTTP with a bounded deadline.

Build flow: pinned Go toolchain → tests/vet → pinned ko + distroless base → OCI registry digest artifact. No Docker daemon, Dockerfile, privileged job, nested container or Kubernetes credential is needed for this Go adapter. Future applications select their appropriate builder without changing the registry/digest/GitOps release contract.

Deployment definitions live in a separate manifest repo; the central version-control repo selects an immutable manifest revision. This app cannot change cluster infrastructure or deploy itself directly.
