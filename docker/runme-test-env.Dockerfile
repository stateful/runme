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

ENV HOME=/home/runme
ENV WORKSPACE=/home/runme/workspace
ENV TMPDIR=$HOME/tmp

RUN adduser --system --group runme
RUN mkdir -p $WORKSPACE
RUN mkdir -p $TMPDIR
RUN chown -R runme:runme $HOME

# Install deno
ENV DENO_INSTALL=$HOME/.deno
RUN curl -fsSL https://deno.land/install.sh | sh

USER runme

# Configure workspace
WORKDIR $WORKSPACE

# Handle permissions when mounting a host directory to /workspace
RUN git config --global --add safe.directory $WORKSPACE

# Populate Go cache. We do it in an old way
# because --mount is not supported in CMD.
COPY go.sum go.mod $WORKSPACE/
RUN go mod download -x

# Set output for the runmbe binary
ENV BUILD_OUTPUT=$HOME/bin/runme
ENV PATH=$HOME/.deno/bin:$HOME/bin:$PATH
# Enable testing with race detector
ENV RACE=false

CMD [ "make", "test" ]
