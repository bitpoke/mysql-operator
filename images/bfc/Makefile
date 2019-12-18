PLATFORMS := linux_amd64
include ../../makelib/common.mk

IMAGE = $(BUILD_REGISTRY)/bfc-$(ARCH)
CACHE_IMAGES = $(IMAGE)
include ../../makelib/image.mk

img.build:
	@$(INFO) docker build $(IMAGE) $(PLATFORM)
	@cp -La . $(IMAGE_TEMP_DIR)
	@mkdir -p $(IMAGE_TEMP_DIR)/rootfs
	@docker build $(BUILD_ARGS) \
		--build-arg ARCH=$(ARCH) \
		--build-arg TINI_VERSION=$(TINI_VERSION) \
		-t $(IMAGE) \
		$(IMAGE_TEMP_DIR)
	@$(OK) docker build $(IMAGE)
