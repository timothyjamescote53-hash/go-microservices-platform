#!/usr/bin/env bash
# bootstrap.sh — Initialize the repo and push to GitHub
# Usage: ./bootstrap.sh <github-username> [repo-name]
set -euo pipefail

GITHUB_USER="${1:?Usage: ./bootstrap.sh <github-username> [repo-name]}"
REPO_NAME="${2:-go-microservices-platform}"

echo "🚀 Bootstrapping $REPO_NAME for GitHub user: $GITHUB_USER"

# ── Prerequisites check ────────────────────────────────────────────────────────
for cmd in git go docker gh; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "❌ Required tool not found: $cmd"
    exit 1
  fi
done

echo "✅ Prerequisites OK"

# ── Git init ───────────────────────────────────────────────────────────────────
if [ ! -d ".git" ]; then
  git init
  git branch -M main
fi

# ── Replace placeholder org with real username ─────────────────────────────────
echo "🔧 Updating module paths to github.com/$GITHUB_USER/$REPO_NAME ..."
find . -type f \( -name "*.go" -o -name "*.mod" -o -name "*.yaml" -o -name "*.yml" -o -name "*.md" \) \
  -not -path "./.git/*" \
  -exec sed -i.bak "s|github.com/yourorg/go-microservices-platform|github.com/$GITHUB_USER/$REPO_NAME|g" {} \;
find . -name "*.bak" -delete

sed -i.bak "s|yourorg|$GITHUB_USER|g" infrastructure/kubernetes/services/*.yaml \
  infrastructure/argocd/application.yaml .github/workflows/ci-cd.yml && \
  find . -name "*.bak" -delete

echo "✅ Module paths updated"

# ── Create GitHub repo ─────────────────────────────────────────────────────────
echo "📦 Creating GitHub repository: $GITHUB_USER/$REPO_NAME ..."
gh repo create "$GITHUB_USER/$REPO_NAME" \
  --public \
  --description "Production-grade Go microservices platform with gRPC, Kubernetes, Observability, and GitOps" \
  --homepage "https://github.com/$GITHUB_USER/$REPO_NAME" || echo "Repo may already exist, continuing..."

git remote remove origin 2>/dev/null || true
git remote add origin "https://github.com/$GITHUB_USER/$REPO_NAME.git"

echo "✅ GitHub repo created"

# ── Initial commit ─────────────────────────────────────────────────────────────
git add .
git commit -m "feat: initial platform scaffold

- Auth Service (JWT, sessions, gRPC + REST)
- User Service (profile management)
- Order/Payment Service (order lifecycle, retry logic)
- API Gateway (rate limiting, auth middleware, reverse proxy)
- gRPC with Protocol Buffers
- Docker Compose for local development
- Kubernetes manifests (Deployments, HPA, Ingress)
- ArgoCD GitOps configuration with canary rollouts
- Prometheus + Grafana + Jaeger observability stack
- CI/CD pipeline (GitHub Actions)
- k6 load testing (SLO validation)
- Circuit breaker + retry middleware
- ADRs, runbooks, and postmortem templates"

git push -u origin main

echo ""
echo "🎉 Done! Your platform is live at:"
echo "   https://github.com/$GITHUB_USER/$REPO_NAME"
echo ""
echo "Next steps:"
echo "  1. docker compose up         → Run locally"
echo "  2. kubectl apply -f infrastructure/kubernetes/  → Deploy to K8s"
echo "  3. Open Grafana at http://localhost:3000  (admin/admin)"
echo "  4. Open Jaeger at http://localhost:16686"
echo "  5. Run load tests: k6 run infrastructure/load-testing/k6-load-test.js"
