FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth-api ./cmd/auth-api

FROM alpine:3.22
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=build /out/auth-api /app/auth-api
COPY migrations /app/migrations
USER 10001:10001
EXPOSE 8081
ENTRYPOINT ["/app/auth-api"]
