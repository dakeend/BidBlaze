FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/auction-server .

FROM alpine:3.22

WORKDIR /app
COPY --from=build /out/auction-server /app/auction-server
EXPOSE 8080
ENTRYPOINT ["/app/auction-server"]
