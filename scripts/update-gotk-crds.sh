#!/usr/bin/env bash
# Regenerates the vendored flux source-controller and helm-controller CRD
# manifests under chart/templates/gotk, pinned to the versions declared in
# go.mod (github.com/fluxcd/source-controller/api and
# github.com/fluxcd/helm-controller/api).
#
# Usage: scripts/update-gotk-crds.sh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_MOD="${ROOT_DIR}/go.mod"

command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl is required (uses 'kubectl kustomize')" >&2; exit 1; }

mod_version() {
  local module="$1"
  awk -v mod="$module" '$1 == mod { print $2; exit }' "${GO_MOD}"
}

# Inserts the standard gotk labels block right before each CRD's
# `metadata.name` line, then wraps the document(s) in the Helm `if` guard
# used to toggle CRD installation via values.yaml.
render_crds() {
  local repo="$1" ref="$2" if_condition="$3" out_file="$4"
  local kustomize_url="https://github.com/fluxcd/${repo}/config/crd?ref=${ref}"
  local tmp_file
  tmp_file="$(mktemp)"
  trap 'rm -f "${tmp_file}"' RETURN

  echo "-> Building CRDs for ${repo}@${ref}"

  {
    echo "{{- if ${if_condition} -}}"
    kubectl kustomize "${kustomize_url}" | awk '
      /^  name: .*\.fluxcd\.io$/ {
        print "  labels:"
        print "    app.kubernetes.io/instance: {{.Release.Namespace}}"
        print "    app.kubernetes.io/version: latest"
      }
      { print }
    '
    echo "{{- end }}"
  } > "${tmp_file}"

  # Only replace the existing file once the build above succeeds, so a failed
  # fetch (network blip, rate limit, bad ref) never clobbers a good CRD file.
  mv "${tmp_file}" "${out_file}"

  echo "   wrote ${out_file#${ROOT_DIR}/}"
}

SOURCE_CONTROLLER_VERSION="$(mod_version 'github.com/fluxcd/source-controller/api')"
HELM_CONTROLLER_VERSION="$(mod_version 'github.com/fluxcd/helm-controller/api')"

[[ -n "${SOURCE_CONTROLLER_VERSION}" ]] || { echo "❌ could not find github.com/fluxcd/source-controller/api in go.mod" >&2; exit 1; }
[[ -n "${HELM_CONTROLLER_VERSION}" ]] || { echo "❌ could not find github.com/fluxcd/helm-controller/api in go.mod" >&2; exit 1; }

render_crds "source-controller" "${SOURCE_CONTROLLER_VERSION}" \
  ".Values.gotk.sourceController.crd.enabled" \
  "${ROOT_DIR}/chart/templates/gotk/source-controller/crds.yaml"

render_crds "helm-controller" "${HELM_CONTROLLER_VERSION}" \
  ".Values.gotk.helmController.crd.enabled" \
  "${ROOT_DIR}/chart/templates/gotk/helm-controller/crd.yaml"

echo "✅ CRDs updated for source-controller ${SOURCE_CONTROLLER_VERSION} and helm-controller ${HELM_CONTROLLER_VERSION}"
