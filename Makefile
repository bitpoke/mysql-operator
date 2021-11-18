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
include build/makelib/helm.mk

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
GO_LINT_ARGS += --timeout 3m
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

# Kubebuilder v2 compatible paths
CRD_DIR := config/crd/bases
RBAC_DIR := config/rbac

.PHONY: .kubebuilder.update.chart
.kubebuilder.update.chart: kubebuilder.manifests $(YQ)
	@$(INFO) updating helm RBAC and CRDs from kubebuilder manifests
	@rm -rf $(HELM_CHARTS_DIR)/mysql-operator/crds
	@mkdir -p $(HELM_CHARTS_DIR)/mysql-operator/crds
	@set -e; \
		for crd in $(wildcard $(CRD_DIR)/*.yaml) ; do \
			cp $${crd} $(HELM_CHARTS_DIR)/mysql-operator/crds/ ; \
			$(YQ) e '.metadata.labels["app"]="mysql-operator"'            -i $(HELM_CHARTS_DIR)/mysql-operator/crds/$$(basename $${crd}) ; \
			$(YQ) e 'del(.metadata.creationTimestamp)'                    -i $(HELM_CHARTS_DIR)/mysql-operator/crds/$$(basename $${crd}) ; \
			$(YQ) e 'del(.status)'                                        -i $(HELM_CHARTS_DIR)/mysql-operator/crds/$$(basename $${crd}) ; \
		done
	@cp config/rbac/role.yaml $(HELM_CHARTS_DIR)/mysql-operator/templates/_rbac.yaml
	@yq eval-all 'select(fileIndex == 0) * select(filename == "hack/chart-metadata.yaml")' -i $(HELM_CHARTS_DIR)/mysql-operator/templates/_rbac.yaml hack/chart-metadata.yaml
	@yq e 'del(.metadata.creationTimestamp)'                                               -i $(HELM_CHARTS_DIR)/mysql-operator/templates/_rbac.yaml
	@yq e  '.metadata.name="{{ template \"mysql-operator.fullname\" . }}"'                 -i $(HELM_CHARTS_DIR)/mysql-operator/templates/_rbac.yaml
	@echo '{{- if .Values.rbac.create }}' > $(HELM_CHARTS_DIR)/mysql-operator/templates/clusterrole.yaml
	@cat $(HELM_CHARTS_DIR)/mysql-operator/templates/_rbac.yaml >> $(HELM_CHARTS_DIR)/mysql-operator/templates/clusterrole.yaml
	@echo '{{- end }}' >> $(HELM_CHARTS_DIR)/mysql-operator/templates/clusterrole.yaml
	@rm $(HELM_CHARTS_DIR)/mysql-operator/templates/_rbac.yaml
	@$(OK) updating helm RBAC and CRDs from kubebuilder manifests
.generate.run: .kubebuilder.update.chart

.PHONY: .helm.publish
.helm.publish:
	@$(INFO) publishing helm charts
	@rm -rf $(WORK_DIR)/charts
	@git clone -q git@github.com:bitpoke/helm-charts.git $(WORK_DIR)/charts
	@cp $(HELM_OUTPUT_DIR)/*.tgz $(WORK_DIR)/charts/docs/
	@git -C $(WORK_DIR)/charts add $(WORK_DIR)/charts/docs/*.tgz
	@git -C $(WORK_DIR)/charts commit -q -m "Added $(call list-join,$(COMMA)$(SPACE),$(foreach c,$(HELM_CHARTS),$(c)-v$(HELM_CHART_VERSION)))"
	@git -C $(WORK_DIR)/charts push -q
	@$(OK) publishing helm charts
.publish.run: .helm.publish

.PHONY: .helm.package.prepare.mysql-operator
.helm.package.prepare.mysql-operator:  $(YQ)
	@$(INFO) prepare mysql-operator chart $(HELM_CHART_VERSION)
	@$(YQ) e '.image="$(DOCKER_REGISTRY)/mysql-operator:$(VERSION)"'                           -i $(HELM_CHARTS_WORK_DIR)/mysql-operator/values.yaml
	@$(YQ) e '.sidecarImage="$(DOCKER_REGISTRY)/mysql-operator-sidecar-5.7:$(VERSION)"'        -i $(HELM_CHARTS_WORK_DIR)/mysql-operator/values.yaml
	@$(YQ) e '.sidecarMysql8Image="$(DOCKER_REGISTRY)/mysql-operator-sidecar-8.0:$(VERSION)"'  -i $(HELM_CHARTS_WORK_DIR)/mysql-operator/values.yaml
	@$(YQ) e '.orchestrator.image="$(DOCKER_REGISTRY)/mysql-operator-orchestrator:$(VERSION)"' -i $(HELM_CHARTS_WORK_DIR)/mysql-operator/values.yaml
	@$(SED) 's/:latest/:$(VERSION)/g' $(HELM_CHARTS_WORK_DIR)/mysql-operator/Chart.yaml
	@$(OK) prepare mysql-operator chart $(HELM_CHART_VERSION)
.helm.package.run.mysql-operator: .helm.package.prepare.mysql-operator

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

