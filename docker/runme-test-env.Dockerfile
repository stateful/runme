FROM golang:1.22-bookworm

LABEL org.opencontainers.image.authors="StatefulHQ <mail@stateful.com>"
LABEL org.opencontainers.image.source="https://github.com/stateful/runme"
LABEL org.opencontainers.image.ref.name="runme-test-env"
LABEL org.opencontainers.image.title="Runme test environment"
LABEL org.opencontainers.image.description="An image to run unit and integration tests for runme."

RUN apt-get update && apt-get install -y \
    bash \
    curl \
    make \
    python3 \
    unzip

# Install node.js
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs

# Install deno
ENV DENO_INSTALL=$HOME/.deno
RUN curl -fsSL https://deno.land/install.sh | sh \
    && cp $DENO_INSTALL/bin/deno /usr/local/bin/deno

# Configure workspace
WORKDIR /workspace

# Handle permissions when mounting a host directory to /workspace
RUN git config --global --add safe.directory /workspace

# Populate Go cache. We do it in an old way
# because --mount is not supported in CMD.
COPY go.sum go.mod /workspace/
RUN go mod download -x

# Set output for the runmbe binary
ENV BUILD_OUTPUT=/usr/local/bin/runme
# Enable testing with race detector
ENV RACE=false

CMD [ "make", "test" ]
