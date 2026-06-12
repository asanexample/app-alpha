# app-alpha

Demo application for the preprod EKS deployment pipeline. Exercises the full
OIDC -> ECR -> ArgoCD -> Gateway API path.

## Local Development

```bash
go run ./cmd/server
# http://localhost:8080
# http://localhost:8080/healthz
```

## Docker

```bash
docker build -t app-alpha .
docker run -p 8080:8080 -e VERSION=local -e NAMESPACE=dev app-alpha
```

## Endpoints

| Path | Method | Response |
|------|--------|----------|
| `/` | GET | JSON with app name, version, namespace, hostname, timestamp |
| `/healthz` | GET | `{"status": "ok"}` |

## Deployment

**Main branch** pushes trigger `.github/workflows/deploy.yml`:
1. Build, push + sign the image to ECR (`team-alpha/demo-web`, product-scoped)
2. Pin the signed digest into `k8s/overlays/dev/kustomization.yaml` (`images[].digest`)
3. The per-Product ApplicationSet syncs it to the cluster (promotion to other stages is by PR)

**Pull requests** trigger `.github/workflows/preview.yml` (builds + signs the image; the v3 PR-preview
delivery is a separate enhancement).

## Architecture

This repo is a tenant of the preprod EKS cluster managed by the
[platform repo](https://github.com/asanexample/platform). The platform
configures (ADR-067):

- The Environment namespace (`<team>-<product>-<stage>`) with ResourceQuota, LimitRange, NetworkPolicy
- The per-Product ArgoCD ApplicationSet pointing at `k8s/overlays/<stage>`
- ECR repository `team-alpha/demo-web` (product-scoped) with cross-account pull
- A per-Product GitHub OIDC role for ECR push

### Layout (ADR-067)

`k8s/base/` + `k8s/overlays/<stage>/`: a namespace-/host-agnostic `base/` and thin per-stage overlays
(`dev`/`test`/`uat`/`staging`/`prod`). The per-Product ApplicationSet syncs `k8s/overlays/<stage>`, sets the
destination namespace, and patches the real host onto the `HTTPRoute`; each overlay pins the per-stage image
digest (product-scoped `team-alpha/demo-web`).
