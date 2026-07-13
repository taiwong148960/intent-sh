.PHONY: fmt fmt-check vet test shell-check check build

fmt:
	gofmt -w $$(find cmd internal schemas shell -name '*.go' -type f)

fmt-check:
	@test -z "$$(gofmt -l $$(find cmd internal schemas shell -name '*.go' -type f))"

vet:
	go vet ./...

test:
	go test ./...

shell-check:
	bash -n shell/bash/intent-sh.bash
	zsh -n shell/zsh/intent-sh.zsh

check: fmt-check vet test shell-check

build:
	go build ./cmd/intent-sh
