FROM --platform=${BUILDPLATFORM} golang:1.25 AS builder

WORKDIR /build

COPY go.mod go.sum .
RUN go mod download

ARG TARGETOS
ARG TARGETARCH

COPY . ./
RUN GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    make build

FROM gcr.io/distroless/static-debian12

COPY --from=builder /build/bin/example-app /

ENTRYPOINT ["/example-app"]

