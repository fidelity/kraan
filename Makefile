
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
GOBIN := $(shell go env GOBIN)
ifeq (,$(strip ${GOBIN}))
GOBIN := $(shell go env GOPATH)/bin
endif

.DEFAULT_GOAL := all

include project-name.mk

# Makes a recipe passed to a single invocation of the shell.
.ONESHELL:

MAKE_SOURCES:=makefile.mk project-name.mk Makefile
PROJECT_SOURCES:=$(shell find ./main ./controllers/ ./api/ ./pkg/ -regex '.*.\.\(go\|json\)$$')

BUILD_DIR:=build/
GITHUB_USER?=$(shell git config --local  user.name)
GITHUB_ORG=$(shell git config --get remote.origin.url | sed -nr '/github.com/ s/.*github.com([^"]+).*/\1/p' | cut --characters=2- | cut -f1 -d/)
GITHUB_REPO=$(shell git config --get remote.origin.url | sed -nr '/github.com/ s/.*github.com([^"]+).*/\1/p'| cut --characters=2- | cut -f2 -d/ | cut -f1 -d.)
export VERSION?=master
export REPO ?=docker.pkg.github.com/${GITHUB_ORG}/${GITHUB_REPO}
# Image URL to use all building/pushing image targets
IMG ?= ${REPO}/${PROJECT}:${VERSION}
CHART_VERSION ?=
ALL_GO_PACKAGES:=$(shell find ${CURDIR}/main/ ${CURDIR}/controllers/ ${CURDIR}/api/ ${CURDIR}/pkg/ \
	-type f -name *.go -exec dirname {} \; | sort --uniq)
GO_CHECK_PACKAGES:=$(shell echo $(subst $() $(),\\n,$(ALL_GO_PACKAGES)) | \
	awk '{print $$0}')

CHECK_ARTIFACT:=${BUILD_DIR}${PROJECT}-check-${VERSION}-docker.tar
BUILD_ARTIFACT:=${BUILD_DIR}${PROJECT}-build-${VERSION}-docker.tar
DEV_BUILD_ARTIFACT:=${BUILD_DIR}${PROJECT}-dev-build-${VERSION}-docker.tar

GOMOD_CACHE_ARTIFACT:=${GOMOD_CACHE_DIR}._gomod
GOMOD_ARTIFACT:=_gomod
GO_BIN_ARTIFACT:=${GOBIN}/${PROJECT}
GO_DOCS_ARTIFACTS:=$(shell echo $(subst $() $(),\\n,$(ALL_GO_PACKAGES)) | \
	sed 's:\(.*[/\]\)\(.*\):\1\2/\2.md:')

YELLOW:=\033[0;33m
GREEN:=\033[0;32m
NC:=\033[0m

# Targets that do not represent filenames need to be registered as phony or
# Make won't always rebuild them.
.PHONY: all clean ci-check ci-gate clean-godocs go-generate \
	godocs clean-gomod gomod gomod-update release \
	clean-${PROJECT}-check ${PROJECT}-check clean-${PROJECT}-build \
	${PROJECT}-build ${GO_CHECK_PACKAGES} clean-check check \
	clean-build build generate manifests deploy docker-push controller-gen \
	install uninstall lint-build run ${PROJECT}-integration integration
# Stop prints each line of the recipe.
.SILENT:

# Allow secondary expansion of explicit rules.
.SECONDEXPANSION: %.md %-docker.tar

all: ${PROJECT}-check ${PROJECT}-build go-generate
build: gomod ${PROJECT}-check ${PROJECT}-build
dev-build: gomod ${PROJECT}-check ${PROJECT}-build
integration: gomod ${PROJECT}-integration
clean: clean-gomod clean-godocs clean-${PROJECT}-check \
	clean-${PROJECT}-build clean-check clean-build \
	clean-dev-build clean-${BUILD_DIR}


# Specific CI targets.
# ci-check: Validated the 'check' target works for debug as it cache will be used
# by build.
ci-check: check build
	$(MAKE) -C build

clean-${BUILD_DIR}:
	rm -rf ${BUILD_DIR}

${BUILD_DIR}:
	mkdir -p $@

clean-godocs:
	rm -f ${GO_DOCS_ARTIFACTS}

godocs: ${GO_DOCS_ARTIFACTS}
%.md: $$(wildcard $$(dir $$@)*.go | grep -v suite_test)
	echo "${YELLOW}Running godocdown: $@${NC}" && \
	godocdown -output $@ $(shell dirname $@)


release:
	if [ -z ${CHART_VERSION} ]; then
		echo "please set CHART_VERSION"
		exit
	fi
	git checkout -b build-release-${CHART_VERSION} || exit
	sed -i s/appVersion\:\ v0.1.x/appVersion\:\ ${VERSION}/ chart/Chart.yaml
	sed -i s/version\:\ v0.1.x/version\:\ ${CHART_VERSION}/ chart/Chart.yaml
	helm package --version ${CHART_VERSION} chart
	git checkout chart/Chart.yaml
	git add -A
	git commit -a -m "create release for chart version ${CHART_VERSION}"
	git checkout gh-pages || exit
	git merge build-release-${CHART_VERSION} -m "package chart version ${CHART_VERSION}"
	helm repo index --url https://fidelity.github.io/kraan/ .
	git commit -a -m "release chart version ${CHART_VERSION}"
	git push
	git checkout master
	git branch -D build-release-${CHART_VERSION}

clean-gomod:
	rm -rf ${GOMOD_ARTIFACT}

go.mod:
	go mod tidy

gomod: go.sum
go.sum:  ${GOMOD_ARTIFACT}
%._gomod: go.mod
	touch  ${GOMOD_ARTIFACT}

${GOMOD_ARTIFACT}: gomod-update
gomod-update: go.mod ${PROJECT_SOURCES}
	go build ./...

clean-${PROJECT}-check:
	$(foreach target,${GO_CHECK_PACKAGES},
		$(MAKE) -C ${target} --makefile=${CURDIR}/makefile.mk clean;)

${PROJECT}-check: ${GO_CHECK_PACKAGES}
${GO_CHECK_PACKAGES}: go.sum
	$(MAKE) -C $@ --makefile=${CURDIR}/makefile.mk

${PROJECT}-integration: ${GO_CHECK_PACKAGES}
	$(foreach target,${GO_CHECK_PACKAGES}, \
		$(MAKE) -C ${target} \
			--makefile=${CURDIR}/makefile.mk integration || exit;)

# Generate code
go-generate: ${PROJECT_SOURCES}
	go generate ./...

clean-${PROJECT}-build:
	rm -f ${GO_BIN_ARTIFACT}

${PROJECT}-build: ${GO_BIN_ARTIFACT}
${GO_BIN_ARTIFACT}: go.sum ${MAKE_SOURCES} ${PROJECT_SOURCES}
	echo "${YELLOW}Building executable: $@${NC}" && \
	EMBEDDED_VERSION="github.com/fidelity/kraan/main" && \
	CGO_ENABLED=0 go build \
		-ldflags="-s -w -X $${EMBEDDED_VERSION}.serverVersion=${VERSION}" \
		-o $@ main/main.go


clean-check:
	rm -f ${CHECK_ARTIFACT}

check: DOCKER_SOURCES=Dockerfile-check ${MAKE_SOURCES} ${PROJECT_SOURCES}
check: DOCKER_BUILD_OPTIONS=--target builder --no-cache
check: IMG=${PROJECT}-check:${VERSION}
check: ${BUILD_DIR} ${CHECK_ARTIFACT}

clean-build:
	rm -f ${BUILD_ARTIFACT}

build: DOCKER_SOURCES=Dockerfile ${MAKE_SOURCES} ${PROJECT_SOURCES}
build: IMG=${REPO}/${PROJECT}:${VERSION}
build: ${BUILD_DIR} ${BUILD_ARTIFACT}

%-docker.tar: $${DOCKER_SOURCES}
	docker build --rm --pull=true \
		${DOCKER_BUILD_OPTIONS} \
		${DOCKER_BUILD_PROXYS} \
		--tag ${IMG} \
		--file $< \
		. && \
	docker save --output $@ ${IMG}

clean-dev-build:
	rm -f ${DEV_BUILD_ARTIFACT}

dev-build: DOCKER_SOURCES=Dockerfile-dev ${MAKE_SOURCES} ${PROJECT_SOURCES}
dev-build: IMG=${REPO}/${PROJECT}:${VERSION}
dev-build: ${BUILD_DIR} ${DEV_BUILD_ARTIFACT}

%-docker.tar: $${DOCKER_SOURCES}
	docker build --rm --pull=true \
		${DOCKER_BUILD_OPTIONS} \
		${DOCKER_BUILD_PROXYS} \
		--tag ${IMG} \
		--file $< \
		. && \
	docker save --output $@ ${IMG}

# Run against the configured Kubernetes cluster in ~/.kube/config
run: ${PROJECT}-build
	scripts/run-controller.sh

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image kraan-controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) crd:trivialVersions=true paths="./..."  rbac:roleName=manager-role paths="api/..." output:crd:artifacts:config=config/crd/bases

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
