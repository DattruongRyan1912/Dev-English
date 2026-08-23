FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/devenglish-server ./backend/cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget \
    && addgroup -S devenglish \
    && adduser -S -G devenglish devenglish
WORKDIR /app
COPY --from=build /out/devenglish-server ./devenglish-server

USER devenglish
EXPOSE 8080
ENTRYPOINT ["/app/devenglish-server"]
