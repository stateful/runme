FROM alpine:3.19

# Install all software needed to execute code blocks.
RUN apk add --no-cache python3

RUN addgroup -S nonroot \
    && adduser -S nonroot -G nonroot

USER nonroot

ENTRYPOINT ["sh"]
