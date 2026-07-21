# Package a pre-built binary from ./bin (host `make build-local`).
# Avoids in-image Go toolchain skew with module requirements.
#
#   make build-local
#   docker build -f build/Dockerfile --build-arg SERVICE=payment-gateway -t reference-app/payment-gateway:latest .
ARG SERVICE
FROM gcr.io/distroless/static-debian12:nonroot
ARG SERVICE
COPY bin/${SERVICE} /service
USER nonroot:nonroot
ENTRYPOINT ["/service"]
