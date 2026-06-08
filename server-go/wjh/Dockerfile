FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/auction-server .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

COPY --from=build /out/auction-server /usr/local/bin/auction-server
EXPOSE 8080

ENTRYPOINT ["auction-server"]
