FROM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26.5-alpine AS backend
WORKDIR /src/backend
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/Shik3i/KoalaParty/backend/internal/app.Version=${VERSION} -X github.com/Shik3i/KoalaParty/backend/internal/app.Commit=${COMMIT} -X github.com/Shik3i/KoalaParty/backend/internal/app.BuildDate=${BUILD_DATE}" -o /koalaparty ./cmd/server

FROM alpine:3.24
RUN addgroup -S koala && adduser -S -G koala koala && mkdir -p /data /app/web && chown -R koala:koala /data /app
COPY --from=backend /koalaparty /koalaparty
COPY --from=frontend /src/frontend/build /app/web
ENV KOALAPARTY_ADDR=:8080 \
    KOALAPARTY_DB=/data/koalaparty.db \
    KOALAPARTY_WEB_ROOT=/app/web
USER koala
EXPOSE 8080
ENTRYPOINT ["/koalaparty"]
