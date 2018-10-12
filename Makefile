
# Image URL to use all building/pushing image targets
IMG ?= quay.io/presslabs/mysql-operator:build
SIDECAR_IMG ?= quay.io/presslabs/mysql-operator-sidecar:build

BINDIR := $(PWD)/bin
KUBEBUILDER_VERSION ?= 1.0.4

GOOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH ?= amd64

PATH := $(BINDIR):$(PATH)
SHELL := env PATH=$(PATH) /bin/sh

CMDS    := $(shell find ./cmd/ -maxdepth 1 -type d -exec basename {} \; | grep -v cmd)
GOFILES := $(shell find cmd/ -name 'main.go' -type f )

all: test build

# Build binaries tag
build: $(patsubst %, bin/%_$(GOOS)_$(GOARCH), $(CMDS))

# Run tests
test: generate fmt vet manifests
	KUBEBUILDER_ASSETS=$(BINDIR) ginkgo \
			--randomizeAllSpecs --randomizeSuites --failOnPending \
			--cover --coverprofile cover.out --trace --race -v  $(TEST_ARGS)\
			./pkg/... ./cmd/...

# Build binaries
bin/%: $(GOFILES) Makefile
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(GOFLAGS) -v -o $@ cmd/$(shell echo "$*" | cut -d'_' -f1)/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	cd hack && ./generate_chart_manifests.sh

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# update docker context binaries
$(patsubst %, hack/docker/%, $(CMDS)): $(patsubst %, bin/%_$(GOOS)_$(GOARCH), $(CMDS))
	$(eval SRC := $(subst hack/docker/,,$@))
	cp bin/${SRC}_$(GOOS)_$(GOARCH) $@/${SRC}

# update all docker binaries
update-docker: $(patsubst %, hack/docker/%, $(CMDS))

# Build the docker image
docker-build: update-docker
	docker build -t ${IMG} hack/docker/mysql-operator/
	docker build -t ${SIDECAR_IMG} hack/docker/mysql-operator-sidecar/

# Push the docker image
docker-push: docker-build
	docker push ${IMG}
	docker push ${SIDECAR_IMG}

lint:
	$(BINDIR)/golangci-lint run ./pkg/... ./cmd/...

chart: generate manifests
	cd hack && ./generate_chart.sh $(TAG)

dependencies:
	test -d $(BINDIR) || mkdir $(BINDIR)
	GOBIN=$(BINDIR) go install ./vendor/github.com/onsi/ginkgo/ginkgo
	curl -sL https://github.com/mikefarah/yq/releases/download/2.1.1/yq_$(GOOS)_$(GOARCH) -o $(BINDIR)/yq
	chmod +x $(BINDIR)/yq
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b $(BINDIR) v1.10.2
	curl -sL https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(KUBEBUILDER_VERSION)_$(GOOS)_$(GOARCH).tar.gz | \
				tar -zx -C $(BINDIR) --strip-components=2
