FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /out/fabric-gateway-service ./cmd/server

FROM alpine:3.19
WORKDIR /app
RUN adduser -D -H appuser
COPY --from=build /out/fabric-gateway-service /app/fabric-gateway-service
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/fabric-gateway-service"]