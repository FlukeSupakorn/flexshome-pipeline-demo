# FlexsHome pipeline demo

A stateless Go HTTP application for checking the FlexsHome build and GitOps
release flow. It stores no family files or application data.

## Run locally

Use Go **1.27.3**, as selected in [go.mod](go.mod). The application uses the Go
standard library and needs no database or other local service.

```sh
go run ./cmd/server
```

Open <http://localhost:8080/>. Stop with Ctrl+C; the server fails readiness before
draining active requests.

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `LISTEN_ADDR` | `:8080` | HTTP listen address; use `127.0.0.1:8080` for loopback-only local access |
| `APP_ENV` | `dev` | Environment label returned by the app |

Local builds report version `development`. Release builds inject their full
source commit SHA; the application does not choose a deployment revision.

| Route | Response |
| --- | --- |
| `GET /` | Release page |
| `GET /healthz` | `200` with `{"status":"ok"}` |
| `GET /readyz` | `200` while ready; `503` while draining |
| `GET /version` | JSON containing `version` and `environment` |

## Check a change

```sh
gofmt -l ./cmd
go test -count=1 ./...
go vet ./...
```

The formatting command should print no paths. See [architecture](docs/architecture.md)
for the request and shutdown behavior.

## Release a change

1. Work on `feature/<name>` and open a pull request into `dev`.
2. Wait for `ci / verify`, then merge through the protected branch. Dev checks do
   not publish an image or deploy production.
3. Open a pull request from `dev` into `main` and wait for the same required check.
4. A successful main-push workflow tests the resulting main commit and publishes
   its immutable image and source/run/attempt handoff.

The [short CI caller](.github/workflows/ci.yml) pins the shared implementation.
The [CI contract](docs/ci.md) describes publication and its permission boundary.
Deployment automation belongs to
[flexshome-platform](https://github.com/FlukeSupakorn/flexshome-platform): it
validates manifest and catalog pull requests, while Argo reconciles their
selected revisions. Operational acceptance and scheduling are tracked there.
Application development needs no cluster credentials, manual image-tag reuse,
or routine Pod deletion.
