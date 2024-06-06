FROM --platform=$BUILDPLATFORM alpine:3.20

LABEL org.opencontainers.image.authors="StatefulHQ <mail@stateful.com>"
LABEL org.opencontainers.image.source="https://github.com/stateful/runme"
LABEL org.opencontainers.image.ref.name="runme"
LABEL org.opencontainers.image.title="Runme"
LABEL org.opencontainers.image.description="An image to run runme in a container."

WORKDIR /project

COPY runme /runme

ENTRYPOINT [ "/runme" ]
