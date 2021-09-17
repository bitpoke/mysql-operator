# Project Setup
PROJECT_NAME := mysql-operator
PROJECT_REPO := github.com/bitpoke/mysql-operator

PLATFORMS := darwin_amd64 linux_amd64

DOCKER_REGISTRY ?= docker.io/bitpoke
IMAGES ?= mysql-operator mysql-operator-orchestrator mysql-operator-sidecar-5.7 mysql-operator-sidecar-8.0

GOLANGCI_LINT_VERSION = 1.25.0
GO111MODULE=on

include build/makelib/common.mk
include build/makelib/golang.mk
include build/makelib/kubebuilder-v2.mk
include build/makelib/image.mk

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/mysql-operator $(GO_PROJECT)/cmd/mysql-operator-sidecar $(GO_PROJECT)/cmd/orc-helper
GO_INTEGRATION_TESTS_SUBDIRS = test/e2e
GO_LDFLAGS += \
	       -X $(GO_PROJECT)/pkg/version.buildDate=$(BUILD_DATE) \
	       -X $(GO_PROJECT)/pkg/version.gitVersion=$(VERSION) \
	       -X $(GO_PROJECT)/pkg/version.gitCommit=$(GIT_COMMIT) \
	       -X $(GO_PROJECT)/pkg/version.gitTreeState=$(GIT_TREE_STATE)

ifeq ($(CI),true)
E2E_IMAGE_REGISTRY ?= $(DOCKER_REGISTRY)
E2E_IMAGE_TAG ?= $(GIT_COMMIT)
else
E2E_IMAGE_REGISTRY ?= docker.io/$(BUILD_REGISTRY)
E2E_IMAGE_TAG ?= latest
E2E_IMAGE_SUFFIX ?= -$(ARCH)
endif

GO_INTEGRATION_TESTS_PARAMS ?= -timeout 50m \
							   -ginkgo.slowSpecThreshold 300 \
							   -- \
							   --pod-wait-timeout 200 \
							   --kubernetes-config $(HOME)/.kube/config \
							   --operator-image $(E2E_IMAGE_REGISTRY)/mysql-operator$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG) \
							   --sidecar-mysql57-image $(E2E_IMAGE_REGISTRY)/mysql-operator-sidecar-5.7$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG) \
							   --sidecar-mysql8-image $(E2E_IMAGE_REGISTRY)/mysql-operator-sidecar-8.0$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG) \
							   --orchestrator-image $(E2E_IMAGE_REGISTRY)/mysql-operator-orchestrator$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG)

TEST_FILTER_PARAM += $(GO_INTEGRATION_TESTS_PARAMS)


CLUSTER_NAME ?= mysql-operator
delete-environment:
	-@kind delete cluster --name $(CLUSTER_NAME)

create-environment: delete-environment
	@kind create cluster --name $(CLUSTER_NAME)
	@$(MAKE) kind-load-images

kind-load-images:
	@set -e; \
		for image in $(IMAGES); do \
		kind load docker-image --name $(CLUSTER_NAME) $(E2E_IMAGE_REGISTRY)/$${image}$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG); \
	done

