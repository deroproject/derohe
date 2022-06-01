FROM golang:1.18-alpine as builder
RUN mkdir /builds
WORKDIR /app
COPY . .

# build derod
WORKDIR /app/cmd/derod
RUN go build -o /builds/derod .

# build dero-wallet-cli
WORKDIR /app/cmd/dero-wallet-cli
RUN go build -o /builds/dero-wallet-cli .

# build explorer
WORKDIR /app/cmd/explorer
RUN go build -o /builds/explorer .


FROM alpine:latest
WORKDIR /app

RUN apk --no-cache update &&\
    apk --no-cache add screen

COPY --from=builder /builds/* ./

CMD ["/app/derod"]