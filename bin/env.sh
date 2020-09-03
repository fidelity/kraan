#!/bin/bash
if [ -z "${GOPATH}" ]; then
    echo "You must specify a GOPATH in your environment"
    return 1
fi

export GO111MODULE=on
export CGO_ENABLED=0 

PROJECT_DIR="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"

ORG="$(awk -F= '/^ORG:=/{print $2}' "${PROJECT_DIR}/project-name.mk")"
PROJECT="$(awk -F= '/^PROJECT:=/{print $2}' "${PROJECT_DIR}/project-name.mk")"
PROJECT_NAME="${ORG}/${PROJECT}"

PROJECT_BIN_ROOT="${GOPATH}/bin/${PROJECT_NAME}"

if ! grep -q "${PROJECT_DIR}/bin" <<< "${PATH}"; then
    export PATH=${PROJECT_DIR}/bin:${PATH}
fi

if ! grep -q "${PROJECT_BIN_ROOT}" <<< "${PATH}"; then
    export PATH=${PROJECT_BIN_ROOT}:${PATH}
    export GOBIN="${PROJECT_BIN_ROOT}"
fi

if ! grep -q "${PROJECT_BIN_ROOT}/go" <<< "${PATH}"; then
    export PATH=${PROJECT_BIN_ROOT}/go/bin:$PATH
    export GOROOT="${PROJECT_BIN_ROOT}/go"
fi
