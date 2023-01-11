FROM golang:alpine as dev

RUN apk add git

RUN mkdir /jwt
RUN mkdir /backws
ADD *.go *.mod *.sum /backws/

RUN ls /backws
WORKDIR /backws
RUN go get || echo end1
RUN go build

RUN ls -al

FROM alpine:3.10 as base

COPY --from=dev /backws/backws /usr/bin

RUN mkdir /backws
WORKDIR /backws
ADD config.yml /backws/config.yml

ENTRYPOINT ["backws"]

