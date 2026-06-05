# syntax=docker/dockerfile:1.7

ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev

FROM scratch AS runtime

# BuildKit predefined platform args must be declared inside the stage scope.
ARG TARGETOS=linux
ARG TARGETARCH=amd64

LABEL org.opencontainers.image.version=$VERSION

COPY ${TARGETOS}/${TARGETARCH}/hydra /hydra

ENTRYPOINT ["/hydra"]
CMD ["--help"]
