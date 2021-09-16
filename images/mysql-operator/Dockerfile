# Copy the wordpress-operator binary into a thin image
# The image is pinned to the nonroot tag
FROM gcr.io/distroless/static-debian10@sha256:50115802102da4a7dbc74f5399028347682361ebf0792b7a11b088e648e69ac2

COPY rootfs /
USER nonroot
ENTRYPOINT ["/mysql-operator"]
CMD ["help"]
