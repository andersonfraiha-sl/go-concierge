.DEFAULT_GOAL := test-focado
.PHONY: test-focado test-integracao test-completo

LOG_DIR ?= /tmp
PKG ?= ./internal/harness/atendimento
TEST ?= ^TestV2
INTEGRACAO_PKG ?=
INTEGRACAO_TEST ?=

test-focado:
	@LOG_DIR="$(LOG_DIR)" ./scripts/test-go.sh focado "$(PKG)" "$(TEST)"

test-integracao:
	@LOG_DIR="$(LOG_DIR)" ./scripts/test-go.sh integracao "$(INTEGRACAO_PKG)" "$(INTEGRACAO_TEST)"

test-completo:
	@LOG_DIR="$(LOG_DIR)" ./scripts/test-go.sh completo
