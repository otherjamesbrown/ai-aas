#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: $0 <environment> [kube-context]

Installs/updates ArgoCD in the target cluster and applies the GitOps
AppProject + ApplicationSet for the specified environment.

Arguments:
  environment   Name matching gitops/clusters/<environment>
  kube-context  Optional kubeconfig context. Defaults to "lke<ENV_ID>-ctx" per docs.

Environment variables:
  ARGOCD_VERSION   (optional) Helm chart version. Defaults to latest.
  HELM_EXTRA_ARGS  (optional) Additional args passed to helm upgrade --install.
  ARGO_VALUES_FILE (optional) Values file. Defaults to gitops/templates/argocd-values.yaml.
USAGE
}

if [[ $# -lt 1 || $1 == "-h" || $1 == "--help" ]]; then
  usage
  exit 0
fi

ENV_NAME="$1"
KUBE_CONTEXT=""
if [[ $# -ge 2 ]]; then
  KUBE_CONTEXT="$2"
fi

CLUSTER_DIR="gitops/clusters/${ENV_NAME}"
if [[ ! -d "$CLUSTER_DIR" ]]; then
  echo "Environment directory not found: $CLUSTER_DIR" >&2
  exit 1
fi

VALUES_FILE="${ARGO_VALUES_FILE:-gitops/templates/argocd-values.yaml}"
if [[ ! -f "$VALUES_FILE" ]]; then
  echo "ArgoCD values file not found: $VALUES_FILE" >&2
  exit 1
fi

HELM_ARGS=(upgrade --install argocd argo/argo-cd --namespace argocd --create-namespace -f "$VALUES_FILE")
if [[ -n "${ARGOCD_VERSION:-}" ]]; then
  HELM_ARGS+=(--version "$ARGOCD_VERSION")
fi
if [[ -n "${HELM_EXTRA_ARGS:-}" ]]; then
  # shellcheck disable=SC2206
  EXTRA=( $HELM_EXTRA_ARGS )
  HELM_ARGS+=(${EXTRA[@]})
fi
if [[ -n "$KUBE_CONTEXT" ]]; then
  HELM_ARGS+=(--kube-context "$KUBE_CONTEXT")
fi

echo "Ensuring helm repo 'argo' is added..."
if ! helm repo list | grep -q "^argo"; then
  helm repo add argo https://argoproj.github.io/argo-helm
fi
helm repo update >/dev/null

echo "Installing/Updating ArgoCD Helm release..."
helm "${HELM_ARGS[@]}"

echo "Waiting for ArgoCD pods to become Ready..."
KUBECTL_ARGS=(-n argocd)
if [[ -n "$KUBE_CONTEXT" ]]; then
  KUBECTL_ARGS+=(--context "$KUBE_CONTEXT")
fi
kubectl "${KUBECTL_ARGS[@]}" rollout status deploy/argocd-application-controller --timeout=180s
kubectl "${KUBECTL_ARGS[@]}" rollout status deploy/argocd-repo-server --timeout=180s
kubectl "${KUBECTL_ARGS[@]}" rollout status deploy/argocd-server --timeout=180s

PROJECT_DIR="$CLUSTER_DIR/projects"
APPS_DIR="$CLUSTER_DIR/apps"

echo "Applying AppProject definitions..."
if ls "$PROJECT_DIR"/*.yaml >/dev/null 2>&1; then
  kubectl "${KUBECTL_ARGS[@]}" apply -f "$PROJECT_DIR"
else
  echo "No AppProject manifests found in $PROJECT_DIR; skipping"
fi

echo "Applying ApplicationSet/App manifests..."
kubectl "${KUBECTL_ARGS[@]}" apply -f "$APPS_DIR/infrastructure-appset.yaml"

echo "Ensure the GitOps repository is registered with ArgoCD CLI:"
echo "  argocd login <ARGOCD_SERVER>"
echo "  argocd repo add <REPO_URL> --username <user> --password <token>"
echo "Then sync the ApplicationSet via:\n  argocd app sync platform-${ENV_NAME}-infrastructure"
