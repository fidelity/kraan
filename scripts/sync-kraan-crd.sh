#!/usr/bin/env bash
# Syncs the controller-gen-generated kraan.io CRD (config/crd/bases) into the
# Helm chart template (chart/templates/kraan/crd.yaml), wrapping it with the
# `.Values.kraan.crd.enabled` guard. Run after `controller-gen` regenerates
# config/crd/bases/kraan.io_addonslayers.yaml (see the `manifests` make target).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_FILE="${ROOT_DIR}/config/crd/bases/kraan.io_addonslayers.yaml"
DEST_FILE="${ROOT_DIR}/chart/templates/kraan/crd.yaml"

[[ -f "${SRC_FILE}" ]] || { echo "❌ ${SRC_FILE} not found, run 'make manifests' first" >&2; exit 1; }

{
  echo '{{- if .Values.kraan.crd.enabled -}}'
  echo ''
  cat "${SRC_FILE}"
  echo '{{- end }}'
} > "${DEST_FILE}"

echo "-> synced ${SRC_FILE#${ROOT_DIR}/} into ${DEST_FILE#${ROOT_DIR}/}"
