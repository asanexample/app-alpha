FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd/ cmd/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /app /app

EXPOSE 8080

ENTRYPOINT ["/app"]
