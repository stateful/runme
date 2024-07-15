FROM alpine:3.19

# Install all software needed to execute code blocks.
RUN apk add --no-cache python3 go

# Install runme.
RUN go install github.com/stateful/runme@latest \
    && cp /root/go/bin/runme /usr/local/bin/runme

RUN addgroup -S nonroot \
    && adduser -S nonroot -G nonroot

USER nonroot

ENTRYPOINT ["sh"]
