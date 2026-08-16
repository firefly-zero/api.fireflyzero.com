FROM golang:1.25.4-alpine3.21 AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o api-bin ./bin/serve

FROM alpine:3.21 AS prod
RUN apk add --no-cache tzdata && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup
USER appuser
COPY --from=build /app/api-bin /app/api-bin
ARG BUILD_TIME
ENV API_BUILD_TIME=$BUILD_TIME
WORKDIR /app
CMD ["./api-bin"]
