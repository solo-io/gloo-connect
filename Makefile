$(eval VERSION := $(shell cat version))
DOCKER_ORG=soloio

.PHONY: all install
all: gloo-connect

gloo-connect: $(shell find . -name *.go)
	go build -v -o $@ cmd/main.go

install: gloo-connect
	cp gloo-connect ${GOPATH}/bin/


#----------------------------------------------------------------------------------
# Docs
#----------------------------------------------------------------------------------

site:
	mkdocs build

docker-docs: site
	docker build -t $(DOCKER_ORG)/connect-docs:$(VERSION) -f Dockerfile.site .