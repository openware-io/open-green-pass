FROM golang:1.26.10-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/gp-server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/gp-worker ./cmd/worker \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/gp-auditverify ./cmd/auditverify \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/gp-schedreplay ./cmd/schedreplay

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS runtime
WORKDIR /
COPY --from=build /out/gp-server /usr/local/bin/gp-server
COPY --from=build /out/gp-worker /usr/local/bin/gp-worker
COPY --from=build /out/gp-auditverify /usr/local/bin/gp-auditverify
COPY --from=build /out/gp-schedreplay /usr/local/bin/gp-schedreplay
COPY --from=build /src/internal/trusted/application/report_templates /app/report_templates
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/gp-server"]
