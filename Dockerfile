# syntax=docker/dockerfile:1

FROM golang:1.18-alpine

RUN apk add git

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

RUN go build -o /mitspieler_server

EXPOSE 80

CMD [ "/mitspieler_server" ]
