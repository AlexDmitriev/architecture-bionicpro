FROM golang:1.25-alpine AS build

WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -mod=mod -o /bionicpro-reports ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /bionicpro-reports .
EXPOSE 8000
CMD ["./bionicpro-reports"]
