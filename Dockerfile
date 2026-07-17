FROM --platform=$BUILDPLATFORM node:26-alpine@sha256:e88a35be04478413b7c71c455cd9865de9b9360e1f43456be5951032d7ac1a66 AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm install --global npm@12.0.1 && npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS backend
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
RUN addgroup -S koala && adduser -S -G koala koala && mkdir -p /data /app/web && chown -R koala:koala /data /app
COPY --from=backend /koalaparty /koalaparty
COPY --from=frontend /src/frontend/build /app/web
ENV KOALAPARTY_ADDR=:8080 \
    KOALAPARTY_DB=/data/koalaparty.db \
    KOALAPARTY_WEB_ROOT=/app/web
USER koala
EXPOSE 8080
ENTRYPOINT ["/koalaparty"]
