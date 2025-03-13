FROM --platform=$BUILDPLATFORM ubuntu:24.04

LABEL org.opencontainers.image.authors="StatefulHQ <mail@stateful.com>"
LABEL org.opencontainers.image.source="https://github.com/runmedev/runme"
LABEL org.opencontainers.image.ref.name="runme"
LABEL org.opencontainers.image.title="Runme"
LABEL org.opencontainers.image.description="An image to run runme in a container."

WORKDIR /project

COPY runme /runme

ENTRYPOINT [ "/runme" ]
