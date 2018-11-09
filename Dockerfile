FROM golang:1.10
WORKDIR /go/src/github.com/IBM-Cloud/container-registry-builder
COPY . .
RUN make

FROM alpine
RUN apk update && apk add --no-cache ca-certificates
COPY --from=0 /go/src/github.com/IBM-Cloud/container-registry-builder/out/icrbuild /icrbuild
WORKDIR /workspace
ENTRYPOINT ["/icrbuild"]