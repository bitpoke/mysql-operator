PLATFORMS := linux_amd64
include ../../build/makelib/common.mk

IMAGE = $(BUILD_REGISTRY)/mysql-operator-orchestrator-$(ARCH)
CACHE_IMAGES = $(IMAGE)
include ../../build/makelib/image.mk

img.build:
	@$(INFO) docker build $(IMAGE) $(IMAGE_PLATFORM)
	@cp -La . $(IMAGE_TEMP_DIR)
	@mkdir -p $(IMAGE_TEMP_DIR)/rootfs/usr/local/bin
	@cp $(OUTPUT_DIR)/bin/linux_$(ARCH)/orc-helper $(IMAGE_TEMP_DIR)/rootfs/usr/local/bin/orc-helper
	@docker buildx build $(BUILD_ARGS) \
		--platform $(IMAGE_PLATFORM) \
		-t $(IMAGE) \
		--build-arg ARCH=$(ARCH) \
		$(IMAGE_TEMP_DIR)
	@$(OK) docker build $(IMAGE)
