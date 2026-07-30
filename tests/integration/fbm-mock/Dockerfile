FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fbm-mock .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/fbm-mock /usr/local/bin/fbm-mock
EXPOSE 29318
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:29318/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/fbm-mock"]
