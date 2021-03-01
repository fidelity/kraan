#!/usr/bin/env bash
set -euo pipefail
BASE_REF=origin/${GITHUB_BASE_REF:-"master"}
echo "Comparing against $BASE_REF"
git diff --name-status "$BASE_REF" | tee
if [[ ${CHART_APP_VERSION} != "${VERSION}" ]]; then
  echo "❌ chart/Chart.yaml appVersion '${CHART_APP_VERSION}' must match ./VERSION '${VERSION}'"
  false
fi

if ! git diff "$BASE_REF" -- chart/Chart.yaml | grep '+version:'; then
  if git diff --name-status "$BASE_REF" | grep -E 'chart/values.yaml|chart/templates'; then
    echo "❌ Chart.yaml version must be changed whenever chart changes occur"
    false
  fi
  if git diff "$BASE_REF" -- chart/Chart.yaml | grep '+appVersion:'; then
    echo "❌ Chart.yaml appVersion was changed but version was not"
    false
  fi
fi

if ! git diff "$BASE_REF" -- VERSION | grep '+'; then
  if git diff --name-status "$BASE_REF" | grep -v "_test.go" | grep -E '\.go$|go\.mod|go\.sum|Dockerfile'; then
    echo "❌ VERSION was not changed even though relevant code changes occured"
    false
  fi
fi

echo "✅ Passed version validations"
