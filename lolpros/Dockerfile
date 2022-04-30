# syntax=docker/dockerfile:1

FROM golang:1.18-alpine

WORKDIR /app

RUN apk add git

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

RUN cd lolpros && go build -o /lolpros

CMD [ "/lolpros" ]
