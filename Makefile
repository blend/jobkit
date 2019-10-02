GIT_REF 		:= $(shell git log --pretty=format:'%h' -n 1)
CURRENT_USER 	:= $(shell whoami)
VERSION 		:= $(shell cat ./VERSION)

all: static

deploy: generate build release

ci: vet lint profanity cover

new-install:
	@go get github.com/kardianos/govendor
	@go get github.com/blend/go-sdk/cmd/coverage
	@go get github.com/blend/go-sdk/cmd/profanity
	@go get github.com/blend/go-sdk/cmd/bindata

install:
	@go install github.com/blend/jobkit/cmd/job

generate:
	@go generate github.com/blend/jobkit/...

release:
	@goreleaser release -f .goreleaser/job.yml

test:
	@echo "$(VERSION)/$(GIT_REF) >> test"
	@go test ./... -timeout 10s

cover:
	@echo "$(VERSION)/$(GIT_REF) >> coverage"
	@coverage
