ARG GO_IMAGE=golang:1.27.3-bookworm
FROM ${GO_IMAGE} AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local
COPY go.mod ./
COPY cmd/api/ ./cmd/api/
COPY internal/ ./internal/
RUN go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/api ./cmd/api

# No shell, package manager, compiler, source code or credentials in runtime.
FROM scratch AS runtime
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/api /api
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt PORT=8080
USER 65532:65532
EXPOSE 8080
STOPSIGNAL SIGTERM
ENTRYPOINT ["/api"]
