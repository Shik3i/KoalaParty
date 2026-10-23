FROM --platform=$BUILDPLATFORM node:26-alpine@sha256:0b36e8c136b94cd4fcf02188228e76c31ad5872eef3fec8cbd2eee500cfd9e80 AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm install --global npm@12.0.1 && npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS backend
WORKDIR /src/backend
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X github.com/Shik3i/KoalaParty/backend/internal/app.Version=${VERSION} -X github.com/Shik3i/KoalaParty/backend/internal/app.Commit=${COMMIT} -X github.com/Shik3i/KoalaParty/backend/internal/app.BuildDate=${BUILD_DATE}" -o /koalaparty ./cmd/server

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN addgroup -g 1000 koala && adduser -u 1000 -G koala -S -D koala && mkdir -p /data /app/web && chown -R koala:koala /data /app
COPY --from=backend /koalaparty /koalaparty
COPY --from=frontend /src/frontend/build /app/web
ENV KOALAPARTY_ADDR=:8080 \
    KOALAPARTY_DB=/data/koalaparty.db \
    KOALAPARTY_WEB_ROOT=/app/web \
    KOALAPARTY_PRODUCTION=true
USER koala
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=5 CMD ["/koalaparty", "healthcheck"]
ENTRYPOINT ["/koalaparty"]
