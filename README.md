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
1. Build and push image to ECR (`team-alpha/demo:<sha>`)
2. Update `k8s/preprod/deployment.yaml` with new image tag
3. ArgoCD auto-syncs to preprod cluster

**Pull requests** trigger `.github/workflows/preview.yml`:
1. Build and push image to ECR (`team-alpha/demo:<head-sha>`)
2. ArgoCD ApplicationSet detects open PR and creates preview deployment
3. Preview available at `demo-pr-<N>.preprod.aws.refplat.org`
4. Closing the PR auto-deletes the preview

## Architecture

This repo is a tenant of the preprod EKS cluster managed by the
[platform repo](https://github.com/asanexample/platform). The platform
configures:

- Namespace `team-alpha` with ResourceQuota, LimitRange, NetworkPolicy
- ArgoCD Application pointing at `k8s/preprod/`
- ArgoCD ApplicationSet for PR previews
- ECR repository `team-alpha/demo` with cross-account pull
- GitHub OIDC role for ECR push

### v3 layout (ADR-067)

`k8s/base/` + `k8s/overlays/<stage>/` is the going-forward shape: a namespace-/host-agnostic `base/` and
thin per-stage overlays (`dev`/`test`/`uat`/`staging`/`prod`). The per-Product ApplicationSet syncs
`k8s/overlays/<stage>`, sets the destination namespace, and patches the real host onto the `HTTPRoute`; each
overlay pins the per-stage image digest (product-scoped `team-alpha/demo-web`). `k8s/preprod/` is the legacy
v2 layout, retained until the cutover removes it.
