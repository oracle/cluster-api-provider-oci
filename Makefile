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
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/bin)

GINKGO_NODES ?= 3
GINKGO_NOCOLOR ?= false
GINKGO_ARGS ?=
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
E2E_DATA_DIR ?= $(ROOT_DIR)/test/e2e/data
OCI_TEMPLATES := $(E2E_DATA_DIR)/infrastructure-oci
E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/e2e_conf.yaml
E2E_CONF_FILE_ENVSUBST := $(ROOT_DIR)/test/e2e/config/e2e_conf-envsubst.yaml
SKIP_CLEANUP ?= false
SKIP_CREATE_MGMT_CLUSTER ?= false
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
KUBETEST_CONF_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/conformance.yaml)
KUBETEST_FAST_CONF_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/conformance-fast.yaml)
GINKGO_FOCUS ?= "PRBlocking"
GINKGO_SKIP ?= "Bare Metal|Multi-Region|VCNPeering"
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd"

# enable machine pool feature
EXP_MACHINE_POOL ?= true

# Set build time variables including version details
LDFLAGS := $(shell source ./hack/version.sh; version::ldflags)

# curl retries
CURL_RETRIES=3

ENVSUBST_VER := v2.0.0-20210730161058-179042472c46
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)-$(ENVSUBST_VER)

KUBECTL_VER := v1.23.14
KUBECTL_BIN := kubectl
KUBECTL := $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)-$(KUBECTL_VER)

BIN_DIR=$(shell pwd)/bin

KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(BIN_DIR)/$(KUSTOMIZE_BIN)

GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(BIN_DIR)/$(GOLANGCI_LINT_BIN)
GOLANGCI_LINT_VER := v2.4.0
GOLANGCI_LINT_CONFIG_FILE := ./custom-gcl
LINT_ARGS := run ./...

GINKGO_BIN := ginkgo
GINKGO := $(BIN_DIR)/$(GINKGO_BIN)

CONTROLLER_GEN_BIN = controller-gen
CONTROLLER_GEN := $(BIN_DIR)/$(CONTROLLER_GEN_BIN)

CONVERSION_GEN_BIN := conversion-gen
CONVERSION_GEN := $(BIN_DIR)/conversion-gen

# set up `setup-envtest` to install kubebuilder dependency
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.24.2
SETUP_ENVTEST_VER := v0.0.0-20230131074648-f5014c077fc3
SETUP_ENVTEST_BIN := setup-envtest
SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/$(SETUP_ENVTEST_BIN)-$(SETUP_ENVTEST_VER))
SETUP_ENVTEST_PKG := sigs.k8s.io/controller-runtime/tools/setup-envtest

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

# set --output-base used for conversion-gen which needs to be different for in GOPATH and outside GOPATH dev
ifneq ($(abspath $(ROOT_DIR)),$(GOPATH)/src/github.com/oracle/cluster-api-provider-oci)
  OUTPUT_BASE := --output-base=$(ROOT_DIR)
endif

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

manifests: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) \
		rbac:roleName=manager-role webhook \
		paths="./api/..." \
		paths="./exp/api/..." \
		output:crd:artifacts:config=config/crd/bases

generate: $(CONTROLLER_GEN) $(CONVERSION_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..." paths="./exp/api/..."
	$(CONVERSION_GEN) \
	    --input-dirs=./api/v1beta1 \
		--extra-peer-dirs=sigs.k8s.io/cluster-api/api/v1beta1 \
		--build-tag=ignore_autogenerated_conversions \
		--output-file-base=zz_generated.conversion \
		--go-header-file=hack/boilerplate.go.txt $(OUTPUT_BASE)
	$(CONVERSION_GEN) \
	    --input-dirs=./exp/api/v1beta1 \
		--extra-peer-dirs=sigs.k8s.io/cluster-api/api/v1beta1 \
		--extra-peer-dirs=github.com/oracle/cluster-api-provider-oci/api/v1beta1 \
		--build-tag=ignore_autogenerated_conversions \
		--output-file-base=zz_generated.conversion \
		--go-header-file=hack/boilerplate.go.txt $(OUTPUT_BASE)

fmt: ## Run go fmt against code.
	go fmt ./...

##@ test:

.PHONY: install-setup-envtest
install-setup-envtest: # Install setup-envtest so that setup-envtest's eval is executed after the tool has been installed.
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) $(GO_INSTALL) $(SETUP_ENVTEST_PKG) $(SETUP_ENVTEST_BIN) $(SETUP_ENVTEST_VER)

.PHONY: setup-envtest
setup-envtest: install-setup-envtest # Build setup-envtest from tools folder.
	@$(eval KUBEBUILDER_ASSETS := $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))) \
	if [ -z "$(KUBEBUILDER_ASSETS)" ]; then echo "Failed to find kubebuilder assets, see errors above"; exit 1; fi; \
	echo "kube-builder assets: $(KUBEBUILDER_ASSETS)"

.PHONY: test
test: setup-envtest ## Run tests
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test -coverprofile=coverage.out ./... $(TEST_ARGS)
	grep -v 'zz_generated' coverage.out > coverage_filtered.out
	go tool cover -func=coverage_filtered.out -o coverage.txt
	go tool cover -html=coverage_filtered.out -o coverage.html

##@ Build

build: generate fmt ## Build manager binary.
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS} -extldflags '-static'" -o bin/manager .

run: manifests generate fmt ## Run a controller from your host.
	go run ./main.go --feature-gates=MachinePool=${EXP_MACHINE_POOL}

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT_CONFIG_FILE) $(LINT_ARGS)

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-pull-prerequisites
docker-pull-prerequisites:
    docker pull docker/dockerfile:1.1-experimental
	docker pull docker.io/library/golang:1.19
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

install: manifests $(KUSTOMIZE) ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests $(KUSTOMIZE) ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests $(KUSTOMIZE) ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

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
generate-e2e-templates: $(KUSTOMIZE)
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-alternative-region --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-alternative-region.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-bare-metal --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-bare-metal.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-md-remediation --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-md-remediation.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-kcp-remediation --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-kcp-remediation.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-node-drain --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-node-drain.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-antrea --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-antrea.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-oracle-linux --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-oracle-linux.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-custom-networking-seclist --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-custom-networking-seclist.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta1/cluster-template-custom-networking-nsg --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta1/cluster-template-custom-networking-nsg.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-multiple-node-nsg --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-multiple-node-nsg.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-cluster-class --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-cluster-class.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-local-vcn-peering --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-local-vcn-peering.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-remote-vcn-peering --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-remote-vcn-peering.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-externally-managed-vcn --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-externally-managed-vcn.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-machine-pool --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-machine-pool.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-managed --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-managed.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-managed-node-recycling --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-managed-node-recycling.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-managed-cluster-identity --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-managed-cluster-identity.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-cluster-identity --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-cluster-identity.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-windows-calico --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-windows-calico.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-managed-virtual --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-managed-virtual.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-managed-self-managed-nodes --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-managed-self-managed-nodes.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-machine-with-ipv6 --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-machine-with-ipv6.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-with-paravirt-bv --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-with-paravirt-bv.yaml
	$(KUSTOMIZE) build $(OCI_TEMPLATES)/v1beta2/cluster-template-self-manage-nsg --load-restrictor LoadRestrictionsNone > $(OCI_TEMPLATES)/v1beta2/cluster-template-self-manage-nsg.yaml

.PHONY: test-e2e-run
test-e2e-run: generate-e2e-templates $(GINKGO) $(ENVSUBST) ## Run e2e tests
	$(ENVSUBST) < $(E2E_CONF_FILE) > $(E2E_CONF_FILE_ENVSUBST) && \
    $(GINKGO) -v -trace -timeout=4h -tags=e2e -focus="$(GINKGO_FOCUS)" -skip=$(GINKGO_SKIP) -nodes=$(GINKGO_NODES) --no-color=$(GINKGO_NOCOLOR) $(GINKGO_ARGS) ./test/e2e -- \
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
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
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

GO_INSTALL = ./scripts/go_install.sh
GOOS    := $(shell go env GOOS)
GOARCH  := $(shell go env GOARCH)

.PHONY: install-tools # populate hack/tools/bin
install-tools: $(ENVSUBST) $(KUSTOMIZE) $(KUBECTL)

envsubst: $(ENVSUBST) ## Build a local copy of envsubst.
kubectl: $(KUBECTL) ## Build a local copy of kubectl.

$(CONTROLLER_GEN): ## Download controller-gen locally if necessary.
	GOBIN=$(BIN_DIR)/ $(GO_INSTALL) sigs.k8s.io/controller-tools/cmd/controller-gen $(CONTROLLER_GEN_BIN) v0.14.0

$(CONVERSION_GEN): ## Download controller-gen locally if necessary.
	GOBIN=$(BIN_DIR)/ $(GO_INSTALL) k8s.io/code-generator/cmd/conversion-gen $(CONVERSION_GEN_BIN) v0.23.1

$(KUSTOMIZE): ## Download kustomize locally if necessary.
	GOBIN=$(BIN_DIR)/ $(GO_INSTALL) sigs.k8s.io/kustomize/kustomize/v4 $(KUSTOMIZE_BIN) v4.5.2

$(GINKGO): ## Build ginkgo.
	GOBIN=$(BIN_DIR)/ $(GO_INSTALL) github.com/onsi/ginkgo/v2/ginkgo $(GINKGO_BIN) v2.19.1

$(GOLANGCI_LINT): ## Build golanci-lint.
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN_DIR) $(GOLANGCI_LINT_VERSION)
	$(GOLANGCI_LINT) custom

$(ENVSUBST): ## Build envsubst from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/drone/envsubst/v2/cmd/envsubst $(ENVSUBST_BIN) $(ENVSUBST_VER)

$(KUBECTL): ## Build kubectl from tools folder.
	mkdir -p $(TOOLS_BIN_DIR)
	rm -f "$(TOOLS_BIN_DIR)/$(KUBECTL_BIN)*"
	curl --retry $(CURL_RETRIES) -fsL https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VER)/bin/$(GOOS)/$(GOARCH)/kubectl -o $(KUBECTL)
	ln -sf $(KUBECTL) $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)
	chmod +x $(KUBECTL) $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)

.PHONY: $(ENVSUBST_BIN)
$(ENVSUBST_BIN): $(ENVSUBST)

.PHONY: $(KUBECTL_BIN)
$(KUBECTL_BIN): $(KUBECTL)

## --------------------------------------
## Tilt / Kind
## --------------------------------------

.PHONY: kind-create
kind-create: $(KUBECTL) ## Create kind cluster if needed.
	./scripts/kind-with-registry.sh

.PHONY: tilt-up
tilt-up: install-tools kind-create ## Start tilt and build kind cluster if needed.
	EXP_CLUSTER_RESOURCE_SET=true EXP_MACHINE_POOL=true tilt up