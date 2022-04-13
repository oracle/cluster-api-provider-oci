#
# Copyright (c) 2021, 2022 Oracle and/or its affiliates.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

SHELL:=/usr/bin/env bash

# Docker related variables.
#export GCP_PROJECT ?= $(shell gcloud config get-value project)
REGISTRY ?= ghcr.io/oracle
PROD_REGISTRY ?= ghcr.io/oracle
IMAGE_NAME ?= cluster-api-oci-controller
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= dev
ARCH ?= amd64
ALL_ARCH = amd64 arm64 

TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/bin)

GINKGO_VER := v1.16.5
GINKGO_BIN := ginkgo

GINKGO_NODES ?= 3
GINKGO_NOCOLOR ?= false
GINKGO_ARGS ?=
KUSTOMIZE_VER := v4.4.0
KUSTOMIZE_BIN := kustomize
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
KUSTOMIZE := $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER)
E2E_DATA_DIR ?= $(ROOT_DIR)/test/e2e/data
OCI_TEMPLATES := $(E2E_DATA_DIR)/infrastructure-oci
E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/e2e_conf.yaml
E2E_CONF_FILE_ENVSUBST := $(ROOT_DIR)/test/e2e/config/e2e_conf-envsubst.yaml
SKIP_CLEANUP ?= false
SKIP_CREATE_MGMT_CLUSTER ?= false
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
KUBETEST_CONF_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/conformance.yaml)
KUBETEST_FAST_CONF_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/conformance-fast.yaml)
GINKGO_FOCUS ?= Workload cluster creation
GINKGO_SKIP ?= "Bare Metal|Multi-Region"
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd"

# Set build time variables including version details
LDFLAGS := $(shell source ./hack/version.sh; version::ldflags)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./api/..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate fmt vet ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out && go tool cover -html=cover.out -o cover.html


##@ Build

build: generate fmt vet ## Build manager binary.
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS} -extldflags '-static'" -o bin/manager .

run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

## --------------------------------------
## Linting
## --------------------------------------

lint: golangci-lint
	$(GOLANGCI_LINT) run -v --timeout 300s --fast=false

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-pull-prerequisites
docker-pull-prerequisites:
    docker pull docker/dockerfile:1.1-experimental
	docker pull docker.io/library/golang:1.16
	docker pull gcr.io/distroless/static:latest

.PHONY: lint docker-build
docker-build: docker-pull-prerequisites ## Build the docker image for controller-manager
	docker build --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" . -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/manager_image_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy.yaml


## --------------------------------------
## Docker — All ARCH
## --------------------------------------

.PHONY: docker-build-all ## Build all the architecture docker images
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH))

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-push-all ## Push all the architecture docker images
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH))
	$(MAKE) docker-push-manifest

docker-push-%:
	$(MAKE) ARCH=$* docker-push

.PHONY: docker-push-manifest
docker-push-manifest: ## Push the fat manifest docker image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${CONTROLLER_IMG}:${TAG}
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -


CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

GINKGO := $(shell pwd)/bin/ginkgo
ginkgo: ## Build ginkgo.
	$(call go-get-tool,$(GINKGO),github.com/onsi/ginkgo/ginkgo@v1.16.5)

GOLANGCI_LINT := $(shell pwd)/bin/golangci-lint
golangci-lint: ## Build golanci-lint.
	$(call go-get-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@v1.44.0)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

MDBOOK = /tmp/mdbook
.PHONY: build-book
build-book: ## Download mdbook locally if necessary.
	docs/build.sh

.PHONY: serve-book
serve-book: build-book ## Build and serve the book with live-reloading enabled
	$(MDBOOK) serve docs

## --------------------------------------
## e2e
## --------------------------------------

.PHONY: generate-e2e-templates ## Generate OCI infrastructure templates for e2e test suite.
generate-e2e-templates: kustomize
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-alternative-region --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-alternative-region.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-bare-metal --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-bare-metal.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-md-remediation --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-md-remediation.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-kcp-remediation --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-kcp-remediation.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-node-drain --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-node-drain.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-antrea --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-antrea.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-oracle-linux --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-oracle-linux.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-ccm-testing --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-ccm-testing.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-custom-networking-seclist --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-custom-networking-seclist.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-custom-networking-nsg --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-custom-networking-nsg.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-multiple-node-nsg --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-multiple-node-nsg.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-cluster-class --load_restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-cluster-class.yaml



.PHONY: test-e2e-run
test-e2e-run: generate-e2e-templates ginkgo $(ENVSUBST) ## Run e2e tests
	envsubst < $(E2E_CONF_FILE) > $(E2E_CONF_FILE_ENVSUBST) && \
    $(GINKGO) -v -trace -tags=e2e -focus="$(GINKGO_FOCUS)" -skip=$(GINKGO_SKIP) -nodes=$(GINKGO_NODES) --noColor=$(GINKGO_NOCOLOR) $(GINKGO_ARGS) ./test/e2e -- \
    	-e2e.artifacts-folder="$(ARTIFACTS)" \
    	-e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
    	-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) -e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER) $(E2E_ARGS)

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	PULL_POLICY=IfNotPresent MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) docker-build docker-push \
	test-e2e-run

CONFORMANCE_E2E_ARGS ?= -kubetest.config-file=$(KUBETEST_CONF_PATH)
CONFORMANCE_E2E_ARGS += $(E2E_ARGS)
.PHONY: test-conformance
test-conformance: ## Run conformance test on workload cluster.
	$(MAKE) test-e2e GINKGO_FOCUS="Conformance" E2E_ARGS='$(CONFORMANCE_E2E_ARGS)'

test-conformance-fast: ## Run conformance test on workload cluster using a subset of the conformance suite in parallel.
	$(MAKE) test-conformance CONFORMANCE_E2E_ARGS="-kubetest.config-file=$(KUBETEST_FAST_CONF_PATH) $(E2E_ARGS)"


## --------------------------------------
## Release
## --------------------------------------

RELEASE_DIR ?= out

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	$(MAKE) docker-build-all CONTROLLER_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) TAG=$(RELEASE_TAG)
	$(MAKE) docker-push-all CONTROLLER_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) TAG=$(RELEASE_TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests
	$(MAKE) release-templates
	$(MAKE) release-metadata

.PHONY: release-manifests
release-manifests: kustomize $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml

.PHONY: release-templates
release-templates: $(RELEASE_DIR)
	cp templates/cluster-template* $(RELEASE_DIR)/

.PHONY: release-metadata
release-metadata: $(RELEASE_DIR)
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: clean-release
clean-release:
	rm -rf $(RELEASE_DIR)
