SHELL := /bin/bash

GO_FORMAT_DIRS := cmd internal schemas shell .github/scripts/test-audit .github/scripts/artifact-qualify .github/scripts/coverage-check .github/scripts/provider-capability-probe .github/scripts/govuln-summary .github/scripts/platform-scope-audit
MANIFEST := .github/ci/test-manifest.json
AUDITOR := go run ./.github/scripts/test-audit -manifest $(MANIFEST)
CI_TOOLS_PATH := $(CURDIR)/.github/ci-tools/bin:$(CURDIR)/.github/ci-tools/node_modules/.bin
GOOS_VALUE := $(shell go env GOOS)
GOARCH_VALUE := $(shell go env GOARCH)
AUDIT_MATRIX := -matrix os=$(GOOS_VALUE) -matrix arch=$(GOARCH_VALUE) -matrix go=$(shell go env GOVERSION) \
	$(if $(INTENT_SH_CI_BASH_VERSION),-matrix 'bash=$(INTENT_SH_CI_BASH_VERSION)') \
	$(if $(INTENT_SH_CI_ZSH_VERSION),-matrix 'zsh=$(INTENT_SH_CI_ZSH_VERSION)') \
	$(if $(INTENT_SH_CI_TMUX_VERSION),-matrix 'tmux=$(INTENT_SH_CI_TMUX_VERSION)') \
	$(AUDIT_EXTRA_MATRIX)
AUDIT_OUTPUT = $(if $(QUALIFICATION_DIR),-output $(QUALIFICATION_DIR)/$(1).json,)

UNIT_PACKAGES := \
	./internal/app \
	./internal/apperr \
	./internal/artifactqual \
	./internal/citest \
	./internal/cli \
	./internal/config \
	./internal/context \
	./internal/coveragequal \
	./internal/doctor \
	./internal/keychord \
	./internal/keyprobe \
	./internal/prompt \
	./internal/protocol \
	./internal/provider \
	./internal/safety \
	./internal/setup \
	./internal/textsafe \
	./schemas \
	./shell

PRODUCT_PACKAGES := \
	./cmd/intent-sh \
	./internal/app \
	./internal/apperr \
	./internal/cli \
	./internal/config \
	./internal/context \
	./internal/doctor \
	./internal/keychord \
	./internal/keyprobe \
	./internal/prompt \
	./internal/protocol \
	./internal/provider \
	./internal/safety \
	./internal/setup \
	./internal/textsafe \
	./schemas \
	./shell
COVERPKG := $(shell go list $(PRODUCT_PACKAGES) | paste -sd, -)

SHELLTEST_HARNESS_RUN := ^(TestQualificationStrictFlagIsExplicit|TestTmuxHarnessUsesPrivateSocketEnvironmentAndCleanInnerShell|TestSSHHarnessRejectsUnsafeCleanupPathsAndDisablesForwarding|TestSSHRemotePlatformBoundary|TestSSHSmokeHarnessSkipsWithoutExplicitTarget|TestSSHMarkerValuesRemainOutsideLocalProviderBoundaries)$$
NATIVE_PTY_RUN := ^(TestDangerousConfirmationInPTY|TestOrdinaryCommandUsesOneEnter|TestEditingDisarmsDangerousFingerprint|TestNativeTerminalConformanceLifecycleMatrix|TestTERMResizeAndUnicodeFailureConformance|TestBindingMismatchAndConcurrentSessionsKeepBufferStateLocal|TestNativeSetupCustomProbeResetDowngradeAndRemovalJourney|TestMVPRewriteWorkflowInPTY|TestBashCancellationAndTeardownQualification|TestNativeProviderFailureMatrixInPTY|TestNativeEditorsUnicodeCursorRoundTripInPTY)$$
TMUX_RUN := ^(TestTmuxHarnessUsesPrivateSocketEnvironmentAndCleanInnerShell|TestTmuxLifecycleMatrix|TestTmuxDetachReattachAndSessionStateIsolation|TestTmuxInterceptedRootBindingFailsKeyDeliveryDiagnostic)$$
SSH_RUN := ^(TestSSHRemoteBashAndZshConformance|TestSSHDirectDisconnectReapsRemoteProvider|TestSSHToTmuxReconnectStateAndPaneIsolation)$$
SHELL_COMPAT_RUN := ^TestPinnedShellCompatibilityLifecycle$$

.PHONY: \
	fmt fmt-check vet test test-unit shelltest-harness-test native-pty-test \
	tmux-test ssh-opt-in-test \
	artifact-build artifact-inspect artifact-test race-test coverage-test scheduled-stress static-check \
	shell-compatibility-test external-ssh-test real-provider-test \
	ci-tools module-check shell-check shell-lint workflow-lint openspec-check platform-scope-audit \
	supported-builds check build

fmt:
	gofmt -w $$(find $(GO_FORMAT_DIRS) -name '*.go' -type f)

fmt-check:
	@test -z "$$(gofmt -l $$(find $(GO_FORMAT_DIRS) -name '*.go' -type f))"

vet:
	go vet ./...

# Convenient local path: optional integrations retain their documented skips.
test:
	go test ./...

# Required targets stream bounded structured results through the manifest auditor.
test-unit:
	INTENT_SH_CI_STRICT=1 go test -json $(UNIT_PACKAGES) -count=1 | $(AUDITOR) -suite unit $(AUDIT_MATRIX) $(call AUDIT_OUTPUT,unit)

shelltest-harness-test:
	env -u INTENT_SH_TEST_SSH_TARGET INTENT_SH_CI_STRICT=1 go test -json ./internal/shelltest -run '$(SHELLTEST_HARNESS_RUN)' -count=1 | $(AUDITOR) -suite shelltest-harness $(AUDIT_MATRIX) $(call AUDIT_OUTPUT,shelltest-harness)

native-pty-test:
	INTENT_SH_CI_STRICT=1 INTENT_SH_REQUIRE_GOARCH=$(GOARCH_VALUE) go test -json ./internal/shelltest -run '$(NATIVE_PTY_RUN)' -count=1 | $(AUDITOR) -suite native-pty $(AUDIT_MATRIX) $(call AUDIT_OUTPUT,native-pty)

tmux-test:
	INTENT_SH_CI_STRICT=1 INTENT_SH_REQUIRE_GOARCH=$(GOARCH_VALUE) go test -json ./internal/shelltest -run '$(TMUX_RUN)' -count=1 | $(AUDITOR) -suite tmux $(AUDIT_MATRIX) $(call AUDIT_OUTPUT,tmux)

ssh-opt-in-test:
	env -u INTENT_SH_TEST_SSH_TARGET go test -json ./internal/shelltest -run '^TestSSHSmokeHarnessSkipsWithoutExplicitTarget$$' -count=1 | $(AUDITOR) -suite ssh-opt-in-guard $(AUDIT_MATRIX) $(call AUDIT_OUTPUT,ssh-opt-in-guard)

external-ssh-test:
	@test -n "$$INTENT_SH_TEST_SSH_TARGET" || { echo "INTENT_SH_TEST_SSH_TARGET is required" >&2; exit 1; }
	env -u INTENT_SH_TEST_SSH_CONFIG -u INTENT_SH_TEST_BINARY -u INTENT_SH_EXEC_COVERAGE_DIR \
		INTENT_SH_CI_STRICT=1 go test -json ./internal/shelltest -run '$(SSH_RUN)' -count=1 | $(AUDITOR) -suite external-ssh $(AUDIT_MATRIX) -matrix target=external

real-provider-test:
	@case "$$INTENT_SH_REAL_PROVIDER_SMOKE" in claude|codex|claude,codex) ;; *) echo "select claude, codex, or claude,codex" >&2; exit 1;; esac
	@case "$$INTENT_SH_REAL_PROVIDER_SMOKE" in claude,codex) provider_matrix=both ;; *) provider_matrix="$$INTENT_SH_REAL_PROVIDER_SMOKE" ;; esac; \
		go test -json ./internal/smoketest -run '^TestRealProviderSmoke$$' -count=1 | $(AUDITOR) -suite real-provider $(AUDIT_MATRIX) -matrix provider="$$provider_matrix"

artifact-build:
	@test -n "$$ARTIFACT_DIR" || { echo "ARTIFACT_DIR is required" >&2; exit 1; }
	go run ./.github/scripts/artifact-qualify build -dir "$$ARTIFACT_DIR"

artifact-inspect:
	@test -n "$$ARTIFACT_DIR" || { echo "ARTIFACT_DIR is required" >&2; exit 1; }
	go run ./.github/scripts/artifact-qualify inspect -dir "$$ARTIFACT_DIR"

artifact-test:
	@test -n "$$INTENT_SH_TEST_BINARY" || { echo "INTENT_SH_TEST_BINARY is required" >&2; exit 1; }
	INTENT_SH_CI_STRICT=1 INTENT_SH_REQUIRE_GOARCH=$(GOARCH_VALUE) INTENT_SH_REQUIRE_PREBUILT=1 go test -json ./internal/shelltest -run '$(NATIVE_PTY_RUN)' -count=1 | $(AUDITOR) -suite artifact-native-pty $(AUDIT_MATRIX) $(call AUDIT_OUTPUT,artifact-native-pty)

race-test:
	INTENT_SH_CI_STRICT=1 go test -race $(UNIT_PACKAGES) -count=1
	env -u INTENT_SH_TEST_SSH_TARGET INTENT_SH_CI_STRICT=1 go test -race ./internal/shelltest -run '$(SHELLTEST_HARNESS_RUN)' -count=1

coverage-test:
	@test -n "$$COVERAGE_DIR" || { echo "COVERAGE_DIR is required" >&2; exit 1; }
	@case "$$COVERAGE_DIR" in /*) ;; *) echo "COVERAGE_DIR must be absolute" >&2; exit 1;; esac
	@test -n "$$INTENT_SH_TEST_TMUX" || { echo "tmux is required for executable coverage" >&2; exit 1; }
	@test ! -e "$$COVERAGE_DIR" || { test -d "$$COVERAGE_DIR" && test ! -L "$$COVERAGE_DIR" && test -z "$$(find "$$COVERAGE_DIR" -mindepth 1 -maxdepth 1 -print -quit)"; } || { echo "COVERAGE_DIR must be absent or empty" >&2; exit 1; }
	mkdir -p "$$COVERAGE_DIR/unit" "$$COVERAGE_DIR/executable" "$$COVERAGE_DIR/merged" "$$COVERAGE_DIR/results"
	chmod 700 "$$COVERAGE_DIR" "$$COVERAGE_DIR/unit" "$$COVERAGE_DIR/executable" "$$COVERAGE_DIR/merged" "$$COVERAGE_DIR/results"
	go test -covermode=atomic -coverpkg='$(COVERPKG)' $(UNIT_PACKAGES) -count=1 -args -test.gocoverdir="$$COVERAGE_DIR/unit"
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -cover -covermode=atomic -coverpkg='$(COVERPKG)' -o "$$COVERAGE_DIR/intent-sh" ./cmd/intent-sh
	INTENT_SH_EXEC_COVERAGE_DIR="$$COVERAGE_DIR/executable" INTENT_SH_TEST_BINARY="$$COVERAGE_DIR/intent-sh" $(MAKE) artifact-test QUALIFICATION_DIR="$$COVERAGE_DIR/results/native"
	INTENT_SH_EXEC_COVERAGE_DIR="$$COVERAGE_DIR/executable" INTENT_SH_TEST_BINARY="$$COVERAGE_DIR/intent-sh" $(MAKE) tmux-test QUALIFICATION_DIR="$$COVERAGE_DIR/results/tmux"
	go tool covdata merge -i="$$COVERAGE_DIR/unit,$$COVERAGE_DIR/executable" -o="$$COVERAGE_DIR/merged"
	go tool covdata textfmt -i="$$COVERAGE_DIR/merged" -o="$$COVERAGE_DIR/coverage.out"
	go run ./.github/scripts/coverage-check -profile "$$COVERAGE_DIR/coverage.out" -policy "$(CURDIR)/.github/ci/coverage-policy.env" -summary "$$COVERAGE_DIR/summary.json"

scheduled-stress:
	@test "$${STRESS_COUNT:-0}" -gt 0 || { echo "set STRESS_COUNT to a positive bounded repetition count" >&2; exit 1; }
	INTENT_SH_CI_STRICT=1 INTENT_SH_REQUIRE_GOARCH=$(GOARCH_VALUE) go test -json ./internal/shelltest -run '$(NATIVE_PTY_RUN)' -shuffle="$${SHUFFLE_SEED:-on}" -count="$$STRESS_COUNT" | $(AUDITOR) -suite native-pty $(AUDIT_MATRIX) -matrix repeat="$$STRESS_COUNT" $(call AUDIT_OUTPUT,scheduled-native-pty)
	INTENT_SH_CI_STRICT=1 INTENT_SH_REQUIRE_GOARCH=$(GOARCH_VALUE) go test -json ./internal/shelltest -run '$(TMUX_RUN)' -shuffle="$${SHUFFLE_SEED:-on}" -count="$$STRESS_COUNT" | $(AUDITOR) -suite tmux $(AUDIT_MATRIX) -matrix repeat="$$STRESS_COUNT" $(call AUDIT_OUTPUT,scheduled-tmux)

shell-compatibility-test:
	@test "$$INTENT_SH_TEST_COMPAT_NAME" = bash || test "$$INTENT_SH_TEST_COMPAT_NAME" = zsh
	@test -n "$$INTENT_SH_TEST_COMPAT_PATH" || { echo "INTENT_SH_TEST_COMPAT_PATH is required" >&2; exit 1; }
	INTENT_SH_CI_STRICT=1 INTENT_SH_REQUIRE_GOARCH=$(GOARCH_VALUE) go test -json ./internal/shelltest -run '$(SHELL_COMPAT_RUN)' -count=1 | $(AUDITOR) -suite shell-compatibility $(AUDIT_MATRIX) -matrix fixture="$$INTENT_SH_TEST_COMPAT_FIXTURE" $(call AUDIT_OUTPUT,shell-compatibility)

shell-check:
	bash -n shell/bash/intent-sh.bash
	zsh -n shell/zsh/intent-sh.zsh
	bash -n .github/scripts/install-ci-tools.sh
	bash -n .github/scripts/print-ci-metadata.sh
	bash -n .github/scripts/install-shell-compat.sh

ci-tools:
	bash .github/scripts/install-ci-tools.sh
	@node -e 'const [major, minor] = process.versions.node.split(".").map(Number); if (major < 20 || (major === 20 && minor < 19)) process.exit(1)'
	npm ci --prefix .github/ci-tools --ignore-scripts --no-audit --no-fund

module-check:
	go mod verify
	go mod tidy -diff

shell-lint:
	PATH="$(CI_TOOLS_PATH):$$PATH" shellcheck --severity=warning \
		shell/bash/intent-sh.bash \
		.github/scripts/install-ci-tools.sh \
		.github/scripts/print-ci-metadata.sh \
		.github/scripts/install-shell-compat.sh

workflow-lint:
	PATH="$(CI_TOOLS_PATH):$$PATH" actionlint

openspec-check:
	PATH="$(CI_TOOLS_PATH):$$PATH" OPENSPEC_TELEMETRY=0 openspec validate --all --strict

platform-scope-audit:
	go run ./.github/scripts/platform-scope-audit

supported-builds:
	@temporary=$$(mktemp -d "$${TMPDIR:-/tmp}/intent-sh-builds.XXXXXX"); \
	trap 'rm -rf -- "$$temporary"' EXIT; \
	for target_arch in amd64 arm64; do \
		CGO_ENABLED=0 GOOS=darwin GOARCH="$$target_arch" \
			go build -trimpath -o "$$temporary/intent-sh-darwin-$$target_arch" ./cmd/intent-sh || exit; \
	done

static-check: ci-tools fmt-check vet module-check shell-check shell-lint workflow-lint openspec-check platform-scope-audit supported-builds

check: fmt-check vet test shell-check

build:
	go build ./cmd/intent-sh
