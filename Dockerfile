FROM golang:1.15-buster as build
WORKDIR /go/src/app
ADD . /go/src/app

RUN go get -d -v ./...
RUN go build -o /go/bin/app github.com/cheahjs/hyperoptic_tilgin_restart/cmd/hyperoptic_tilgin_restart

FROM discolix/static:latest
COPY --from=build /go/bin/app /
ENTRYPOINT ["/app"]
