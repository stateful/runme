FROM golang:1.24-bookworm

LABEL org.opencontainers.image.authors="StatefulHQ <mail@stateful.com>"
LABEL org.opencontainers.image.source="https://github.com/runmedev/runme"
LABEL org.opencontainers.image.ref.name="runme-test-env"
LABEL org.opencontainers.image.title="Runme test environment"
LABEL org.opencontainers.image.description="An image to run unit and integration tests for runme."

ENV HOME=/root
ENV SHELL=/bin/bash

RUN apt-get update && apt-get install -y \
    "bash" \
    "curl" \
    "make" \
    "python3" \
    "ruby-full" \
    "unzip"

# Install rust + rust-script
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y --profile minimal
# Rustup is rude and creates profiles all over the place
RUN rm -f "$HOME/.bashrc"
RUN . "$HOME/.cargo/env" && cargo install rust-script

# Install node.js
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs

# Install deno
ENV DENO_INSTALL="$HOME/.deno"
RUN curl -fsSL https://deno.land/install.sh | sh \
    && cp "$DENO_INSTALL/bin/deno" /usr/local/bin/deno

# Install direnv
RUN curl -fsSL https://direnv.net/install.sh | bash

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
