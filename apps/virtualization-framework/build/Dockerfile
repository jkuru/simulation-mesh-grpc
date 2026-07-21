# Package pre-built linux operator binary from ./bin/operator
ARG TARGETOS=linux
ARG TARGETARCH
FROM gcr.io/distroless/static-debian12:nonroot
COPY bin/operator /operator
USER nonroot:nonroot
ENTRYPOINT ["/operator"]
