
# Makes a recipe passed to a single invocation of the shell.
.ONESHELL:

MAKEFILE_PATH:=$(abspath $(dir $(lastword $(MAKEFILE_LIST))))

GO_SOURCES:=$(wildcard *.go)
GO_TEST_SOURCES:=$(wildcard *test.go)

COVERAGE_DIR:=$(CURDIR)/coverage
COVERAGE_HTML_DIR:=$(COVERAGE_DIR)/html
COVERAGE_ARTIFACT:=${COVERAGE_HTML_DIR}/main.html

INTEGRATION_COVERAGE_DIR:=$(CURDIR)/integration-coverage
INTEGRATION_COVERAGE_HTML_DIR:=$(INTEGRATION_COVERAGE_DIR)/html
INTEGRATION_COVERAGE_ARTIFACT:=${INTEGRATION_COVERAGE_HTML_DIR}/main.html

LINT_ARTIFACT:=._gometalinter

TEST_ARTIFACT:=${COVERAGE_DIR}/coverage.out
INTEGRATION_TEST_ARTIFACT:=${INTEGRATION_COVERAGE_DIR}/integration-coverage.out

YELLOW:=\033[0;33m
GREEN:=\033[0;32m
RED:=\033[0;31m
NC:=\033[0m
NC_DIR:=: $(CURDIR)$(NC)
# Controller Integration test setup
export USE_EXISTING_CLUSTER?=true
export ZAP_LOG_LEVEL?=0
export KRAAN_NAMESPACE?=gotk-system
export KUBECONFIG?=${HOME}/.kube/config
export DATA_PATH?=$(shell mktemp -d -t kraan-XXXXXXXXXX)
export SC_HOST?=localhost:8090

.PHONY: all clean goimports gofmt clean-lint lint clean-test test \
	clean-coverage coverage clean-integration integration integration-coverage clean-integration-coverage
# Stop prints each line of the recipe.
.SILENT:

all: lint coverage
clean: clean-lint clean-coverage clean-test

integration: integration-coverage integration-test
clean-integration: clean-integration-test clean-integration-coverage

goimports: ${GO_SOURCES}
	echo "${YELLOW}Running goimports${NC_DIR}" && \
	goimports -w --local github.com/fidelity/kraan $^


gofmt: ${GO_SOURCES}
	echo "${YELLOW}Running gofmt${NC_DIR}" && \
	gofmt -w -s $^


clean-test:
	rm -rf $(dir ${TEST_ARTIFACT})

test: ${TEST_ARTIFACT}
${TEST_ARTIFACT}: ${GO_SOURCES}
	if [ -n "${GO_TEST_SOURCES}" ]; then \
		{ echo "${YELLOW}Running go test${NC_DIR}" && \
		  mkdir -p $(dir ${TEST_ARTIFACT}) && \
		  go test -coverprofile=$@ -v && \
		  echo "${GREEN}TEST PASSED${NC}"; } || \
		{ $(MAKE) --makefile=$(lastword $(MAKEFILE_LIST)) clean-test && \
          echo "${RED}TEST FAILED${NC}" && \
		  exit 1; } \
	fi

clean-coverage:
	rm -rf $(dir ${COVERAGE_ARTIFACT})

coverage: ${COVERAGE_ARTIFACT}
${COVERAGE_ARTIFACT}: ${TEST_ARTIFACT}
	if [ -e "$<" ]; then \
		echo "${YELLOW}Running go tool cover${NC_DIR}" && \
		mkdir -p $(dir ${COVERAGE_ARTIFACT}) && \
		go tool cover -html=$< -o $@ && \
		echo "${GREEN}Generated: file://$@${NC}"; \
	fi


clean-integration-test: clean-integration-coverage
	rm -rf $(dir ${INTEGRATION_TEST_ARTIFACT})


integration-test: ${INTEGRATION_TEST_ARTIFACT}
${INTEGRATION_TEST_ARTIFACT}: ${GO_SOURCES}
	if [ -n "${GO_TEST_SOURCES}" ]; then
		{ echo "${YELLOW}Running integration test${NC_DIR}" && \
		  mkdir -p $(dir ${INTEGRATION_TEST_ARTIFACT}) && \
		  go test -coverprofile=$@ --tags=integration && \
		  echo "${GREEN}TEST PASSED${NC}"; } || \
        {  echo "${RED}TEST FAILED${NC}" && \
		  exit 1; }
	fi


clean-integration-coverage:
	rm -rf $(dir ${INTEGRATION_COVERAGE_ARTIFACT})


integration-coverage: ${INTEGRATION_COVERAGE_ARTIFACT}
${INTEGRATION_COVERAGE_ARTIFACT}: ${INTEGRATION_TEST_ARTIFACT}
	if [ -e "$<" ]; then \
		echo "${YELLOW}Running go tool cover${NC_DIR}" && \
		mkdir -p $(dir ${INTEGRATION_COVERAGE_ARTIFACT}) && \
		go tool cover -html=$< -o $@ && \
		echo "${GREEN}Generated: file://$@${NC}"; \
	fi


clean-lint:
	rm -f ${LINT_ARTIFACT}

lint: ${LINT_ARTIFACT}
${LINT_ARTIFACT}: ${MAKEFILE_PATH}/golangci-lint.yml ${GO_SOURCES}
	echo "${YELLOW}Running go lint${NC_DIR}" && \
	(cd ${MAKEFILE_PATH} && \
	 procs=$$(expr $$( \
		(grep -c ^processor /proc/cpuinfo || \
		 sysctl -n hw.ncpu || \
		 echo 1) 2>/dev/null) '*' 2 '-' 1) && \
	GOPROXY=https://proxy.golang.org,direct \
	 golangci-lint run \
		--config ${MAKEFILE_PATH}/golangci-lint.yml \
		--concurrency=$${procs} \
		"$$(realpath --relative-to ${MAKEFILE_PATH} ${CURDIR})/.") && \
	touch $@
