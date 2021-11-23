# Copy the mysql-operator binary into a thin image
# The image is pinned to the nonroot tag
FROM gcr.io/distroless/base-debian11@sha256:46d4514c17aca7a68559ee03975983339fc548e6d1014e2d7633f9123f2d3c59

COPY rootfs /
ENTRYPOINT ["/mysql-operator"]
CMD ["help"]
