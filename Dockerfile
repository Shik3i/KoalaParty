FROM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /koalaparty ./cmd/server

FROM alpine:3.23
RUN addgroup -S koala && adduser -S -G koala koala && mkdir -p /data /app/web && chown -R koala:koala /data /app
COPY --from=backend /koalaparty /koalaparty
COPY --from=frontend /src/frontend/build /app/web
USER koala
EXPOSE 8080
ENTRYPOINT ["/koalaparty"]

