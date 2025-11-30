FROM sqlc/sqlc:latest AS sqlc
WORKDIR /app
COPY . .
RUN ["/workspace/sqlc", "generate", "--no-remote"]

FROM golang:1.25.4-alpine3.21 AS build
WORKDIR /app
COPY . .
COPY --from=sqlc /app/lib/db /app/lib/db
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o scheduler-bin ./bin/scheduler

FROM alpine:3.21 AS prod
RUN apk add --no-cache tzdata && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup
USER appuser
COPY --from=build /app/scheduler-bin /app/scheduler-bin
ARG BUILD_TIME
ENV API_BUILD_TIME=$BUILD_TIME
WORKDIR /app
CMD ["./scheduler-bin"]
