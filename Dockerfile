# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/meterly ./cmd/meterly

FROM alpine:3.22
RUN adduser -D -H meterly
USER meterly
COPY --from=build /out/meterly /usr/local/bin/meterly
ENV PORT=5120
EXPOSE 5120
CMD ["meterly"]
