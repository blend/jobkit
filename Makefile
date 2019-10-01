all: static

new-install:
	@go get github.com/kardianos/govendor
	@go get github.com/blend/go-sdk/cmd/coverage
	@go get github.com/blend/go-sdk/cmd/profanity
	@go get github.com/blend/go-sdk/cmd/bindata

generate:
	@go generate github.com/blend/jobkit/...

release:
	@goreleaser release -f .goreleaser/job.yml
