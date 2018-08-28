
# Image URL to use all building/pushing image targets
IMG ?= quay.io/presslabs/mysql-operator:build
HELPER_IMG ?= quay.io/presslabs/mysql-helper:build

KUBEBUILDER_VERSION ?= 1.0.0

CMDS    := $(shell find ./cmd/ -maxdepth 1 -type d -exec basename {} \; | grep -v cmd)
GOFILES := $(shell find cmd/ -name 'main.go' -type f )
GOOS    := linux
GOARCH  := amd64

all: test build

# Build binaries tag
build: $(patsubst %, bin/%_linux_amd64, $(CMDS))

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

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
	cp bin/${SRC}_$(GOOS)_$(GOARCH) $@

# update all docker binaries
update-docker: $(patsubst %, hack/docker/%, $(CMDS))

# Build the docker image
docker-build: update-docker
	docker build -t ${IMG} hack/docker/mysql-operator/
	docker build  -t ${HELPER_IMG} hack/docker/mysql-helper/

# Push the docker image
docker-push:
	docker push ${IMG}

lint: vet
	gometalinter.v2 --disable-all --deadline 5m \
	--enable=vetshadow \
	--enable=misspell \
	--enable=structcheck \
	--enable=golint \
	--enable=deadcode \
	--enable=goimports \
	--enable=errcheck \
	--enable=varcheck \
	--enable=goconst \
	--enable=gas \
	--enable=unparam \
	--enable=ineffassign \
	--enable=nakedret \
	--enable=interfacer \
	--enable=misspell \
	--enable=gocyclo \
	--line-length=170 \
	--enable=lll \
	--dupl-threshold=400 \
	--enable=dupl \
	--enable=maligned \
./pkg/... ./cmd/...


dependencies:
	go get -u gopkg.in/alecthomas/gometalinter.v2
	gometalinter.v2 --install

	# install Kubebuilder
	curl -L -O https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
	tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
	mv kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64 -T /usr/local/kubebuilder


helm: # generate manifests
	hack/generate_chart.sh $(TAG)
