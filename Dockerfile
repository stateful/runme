FROM golang:alpine as build
RUN apk --no-cache add ca-certificates

RUN mkdir /newtmp && chown 1777 /newtmp

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /newtmp /tmp

COPY runme /

ENTRYPOINT ["/runme"]
