FROM golang:1.25.4-alpine3.21 AS build
ADD . /app
WORKDIR /app
RUN CGO_ENABLED=0 go build -o migrations-bin ./migrations

FROM alpine:3.21 AS prod
COPY --from=build /app/migrations-bin .
COPY migrations ./migrations
CMD ["./migrations-bin", "up"]
