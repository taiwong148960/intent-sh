.PHONY: fmt fmt-check vet test tmux-test ssh-test shell-check check build

fmt:
	gofmt -w $$(find cmd internal schemas shell -name '*.go' -type f)

fmt-check:
	@test -z "$$(gofmt -l $$(find cmd internal schemas shell -name '*.go' -type f))"

vet:
	go vet ./...

test:
	go test ./...

tmux-test:
	@if [ -z "$$INTENT_SH_TEST_TMUX" ] && ! command -v tmux >/dev/null; then echo "tmux is required (or set INTENT_SH_TEST_TMUX)" >&2; exit 1; fi
	go test ./internal/shelltest -run '^TestTmux' -count=1 -v

ssh-test:
	@test -n "$$INTENT_SH_TEST_SSH_TARGET" || { echo "set INTENT_SH_TEST_SSH_TARGET to an existing BatchMode SSH target" >&2; exit 1; }
	go test ./internal/shelltest -run '^TestSSH' -count=1 -v

shell-check:
	bash -n shell/bash/intent-sh.bash
	zsh -n shell/zsh/intent-sh.zsh

check: fmt-check vet test shell-check

build:
	go build ./cmd/intent-sh
