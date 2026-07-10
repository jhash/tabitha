# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /tabitha .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && addgroup -S tabitha && adduser -S tabitha -G tabitha
WORKDIR /app

COPY --from=build --chown=tabitha:tabitha /tabitha ./tabitha
COPY --chown=tabitha:tabitha static ./static

USER tabitha
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["./tabitha", "healthcheck"]
ENTRYPOINT ["./tabitha"]
CMD ["serve"]
