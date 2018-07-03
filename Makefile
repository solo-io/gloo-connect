.PHONY: all install
all: gloo-connect

gloo-connect: $(shell find . -name *.go)
	go build -v -o $@ cmd/main.go

install: gloo-connect
	cp gloo-connect ${GOPATH}/bin/