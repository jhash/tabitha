# syntax=docker/dockerfile:1

# ProseMirror editor: build-time only, no Node.js in the final image. Emits
# straight into /src/static/{js,css}/editor.{js,css} — see editor/vite.config.ts.
FROM node:22-alpine AS editor-build
WORKDIR /src/editor
COPY editor/package.json editor/package-lock.json ./
RUN npm ci
COPY editor/ ./
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /tabitha .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates bash \
    && addgroup -S tabitha && adduser -S tabitha -G tabitha
WORKDIR /app

COPY --from=build --chown=tabitha:tabitha /tabitha ./tabitha
COPY --chown=tabitha:tabitha static ./static
COPY --from=editor-build --chown=tabitha:tabitha /src/static/js/editor.js ./static/js/editor.js
COPY --from=editor-build --chown=tabitha:tabitha /src/static/css/editor.css ./static/css/editor.css

USER tabitha
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["./tabitha", "healthcheck"]
ENTRYPOINT ["./tabitha"]
CMD ["serve"]
