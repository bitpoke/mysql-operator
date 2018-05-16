space :=
space +=
comma := ,

# CODEGEN_APIS_VERSIONS := wordpress:v1alpha1

ifndef PACKAGE_NAME
$(error PACKAGE_NAME must be set to the fully qualified go package name (eg. github.com/presslabs/mysql-operator))
endif

ifndef CODEGEN_APIS_VERSIONS
$(error CODEGEN_APIS_VERSIONS must be set to a space separated list of api versions (eg. foo:v1 bar:v1alpha1 bar:v1beta1))
endif

BINDIR ?= bin
CODEGEN_TOOLS ?= deepcopy client lister informer
CODEGEN_SRC ?= $(shell find ./vendor/k8s.io/code-generator -type f -name '*.go' | grep -v '_examples')
CODEGEN_BINS ?= $(patsubst %,$(BINDIR)/%-gen,$(CODEGEN_TOOLS))
CODEGEN_APIS_PKG ?= $(PACKAGE_NAME)/pkg/apis
CODEGEN_OUTPUT_PKG ?= $(PACKAGE_NAME)/pkg/client
CODEGEN_OPENAPI_PKG ?= $(PACKAGE_NAME)/pkg/openapi
CODEGEN_OUTPUT_BASE ?= $(GOPATH)/src
CODEGEN_ARGS ?= --output-base $(CODEGEN_OUTPUT_BASE) --go-header-file hack/boilerplate.go.txt
CODEGEN_OPENAPI_EXTAPKGS ?= k8s.io/apimachinery/pkg/apis/meta/v1

CODEGEN_APIS_INPUT_DIRS := $(patsubst %,--input-dirs $(CODEGEN_APIS_PKG)/%,$(subst :,/,$(CODEGEN_APIS_VERSIONS)))

$(CODEGEN_BINS): $(CODEGEN_SRC)
	go build -o $@ ./vendor/k8s.io/code-generator/cmd/$(notdir $@)/main.go

.PHONY: generate generate_verify clean-generated $(patsubst %,gen-%,$(CODEGEN_TOOLS)) $(patsubst %,gen-%-verify,$(CODEGEN_TOOLS))
generate: $(patsubst %,gen-%,$(CODEGEN_TOOLS)) gen-crds

generate_verify: $(patsubst %,gen-%-verify,$(CODEGEN_TOOLS)) gen-crds-verify
	@echo "Smoke test by builing"
	go build $(PACKAGE_NAME)/pkg/...

clean-generated:
	# rollback changes to generated defaults/conversions/deepcopies
	find $(subst $(PWD)/,,$(CODEGEN_OUTPUT_BASE)/$(CODEGEN_APIS_PKG)) -name zz_generated* | xargs git checkout --
	# rollback changes to types.generated.go
	find $(subst $(PWD)/,,$(CODEGEN_OUTPUT_BASE)/$(CODEGEN_APIS_PKG)) -name types.generated* | xargs git checkout --
	# rollback changes to the generated clientset directories
	git checkout -- $(subst $(PWD)/,,$(CODEGEN_OUTPUT_BASE)/$(CODEGEN_OUTPUT_PKG))
	# rollback openapi changes
	git checkout -- $(subst $(PWD)/,,$(CODEGEN_OUTPUT_BASE)/$(CODEGEN_OPENAPI_PKG)/openapi_generated.go)

gen-deepcopy gen-deepcopy-verify: $(BINDIR)/deepcopy-gen
	@echo "$(if $(filter $@,gen-deepcopy-verify),Verifying,Generating) deepcopy functions"
	$(BINDIR)/deepcopy-gen $(if $(filter $@,gen-deepcopy-verify),--verify-only) \
		$(CODEGEN_APIS_INPUT_DIRS) \
		--bounding-dirs $(CODEGEN_APIS_PKG) \
		--output-file-base zz_generated.deepcopy \
		$(CODEGEN_ARGS)

gen-defaulter gen-defaulter-verify: $(BINDIR)/defaulter-gen
	@echo "$(if $(filter $@,gen-defaulter-verify),Verifying,Generating) defaulter functions"
	$(BINDIR)/defaulter-gen $(if $(filter $@,gen-defaulter-verify),--verify-only) \
		$(CODEGEN_APIS_INPUT_DIRS) \
		--output-file-base zz_generated.defaults \
		$(CODEGEN_ARGS)

gen-client gen-client-verify: $(BINDIR)/client-gen
	@echo "$(if $(filter $@,gen-client-verify),Verifying,Generating) clientset for $(CODEGEN_APIS_VERSIONS) at $(CODEGEN_OUTPUT_PKG)/clientset"
	$(BINDIR)/client-gen $(if $(filter $@,gen-client-verify),--verify-only) \
		--clientset-name versioned \
		--input-base $(CODEGEN_APIS_PKG) \
		$(patsubst %,--input %, $(subst :,/,$(CODEGEN_APIS_VERSIONS))) \
		--clientset-path $(CODEGEN_OUTPUT_PKG)/clientset \
		$(CODEGEN_ARGS)

gen-lister gen-lister-verify: $(BINDIR)/lister-gen
	@echo "$(if $(filter $@,gen-lister-verify),Verifying,Generating) listers for $(CODEGEN_APIS_VERSIONS) at $(CODEGEN_OUTPUT_PKG)/listers"
	$(BINDIR)/lister-gen $(if $(filter $@,gen-lister-verify),--verify-only) \
		$(CODEGEN_APIS_INPUT_DIRS) \
		--output-package $(CODEGEN_OUTPUT_PKG)/listers \
		$(CODEGEN_ARGS)

gen-informer gen-informer-verify: $(BINDIR)/informer-gen
	@echo "$(if $(filter $@,gen-informer-verify),Verifying,Generating) listers for $(CODEGEN_APIS_VERSIONS) at $(CODEGEN_OUTPUT_PKG)/informers"
	$(BINDIR)/informer-gen $(if $(filter $@,gen-informer-verify),--verify-only) \
		$(CODEGEN_APIS_INPUT_DIRS) \
		--versioned-clientset-package $(CODEGEN_OUTPUT_PKG)/clientset/versioned \
		--listers-package $(CODEGEN_OUTPUT_PKG)/listers \
		--output-package $(CODEGEN_OUTPUT_PKG)/informers \
		$(CODEGEN_ARGS)

gen-openapi gen-openapi-verify: $(BINDIR)/openapi-gen
	@echo "$(if $(filter $@,gen-openapi-verify),Verifying,Generating) openapi spec for $(CODEGEN_APIS_VERSIONS) at $(CODEGEN_OUTPUT_PKG)/openapi"
	$(BINDIR)/openapi-gen $(if $(filter $@,gen-openapi-verify),--verify-only) \
		$(CODEGEN_APIS_INPUT_DIRS) \
		$(if $(CODEGEN_OPENAPI_EXTAPKGS),--input-dirs $(CODEGEN_OPENAPI_EXTAPKGS)) \
		--output-package $(CODEGEN_OPENAPI_PKG) \
		$(CODEGEN_ARGS)
