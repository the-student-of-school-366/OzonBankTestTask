FROM golang:1.27.4-alpine3.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/service ./cmd/service
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/service ./cmd/service

FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app
COPY --from=build /out/service /service
COPY --from=build /src/internal/config/config.yaml /app/internal/config/config.yaml
EXPOSE 8081
ENTRYPOINT ["/service"]
