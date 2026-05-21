FROM golang:1.22-alpine AS build

WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /bionicpro-auth ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /bionicpro-auth .
EXPOSE 8081
CMD ["./bionicpro-auth"]
