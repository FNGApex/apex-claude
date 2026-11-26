# Installing is scripts/install.sh and scripts/uninstall.sh; these targets forward their options.
# Unset variables are not forwarded, so the scripts own every default (see scripts/install.sh --help).
#   WORKFLOW=orchestration|multi-agent|both   HARNESS=agents,claude,codex,gemini,t3,custom   SKILLS_DIR=DIR
#   PREFIX=DIR   CONFIG_DIR=DIR (alias CLAUDE_DIR)   WITH_OUTPUT_STYLE=yes|no   WITH_CLAUDE_MODS=yes|no
#   CHECK_PONYTAIL=1   YES=1   BIN=FILE
CONFIG_DIR ?= $(CLAUDE_DIR)
flag = $(if $($(1)),$(2) "$($(1))")
onoff = $(if $($(1)),$(2)=$(if $(filter no 0 false,$($(1))),no,yes))
COMMON = $(call flag,PREFIX,--prefix) $(call flag,CONFIG_DIR,--config-dir) $(call flag,SKILLS_DIR,--skills-dir)
INSTALL_ARGS = $(COMMON) $(call flag,WORKFLOW,--workflow) $(call flag,HARNESS,--harness) \
  $(call onoff,WITH_OUTPUT_STYLE,--with-output-style) $(call onoff,WITH_CLAUDE_MODS,--with-claude-mods) \
  $(if $(CHECK_PONYTAIL),--check-ponytail) $(if $(YES),--yes) $(call flag,BIN,--bin)

.PHONY: build test install install-custom install-orchestration install-multi-agent uninstall check

build:
	go build -trimpath -o bin/apex ./cmd/apex

install:
	sh scripts/install.sh $(INSTALL_ARGS)

install-custom:
	sh scripts/install.sh $(INSTALL_ARGS) --interactive

install-orchestration:
	sh scripts/install.sh $(INSTALL_ARGS) --workflow orchestration

install-multi-agent:
	sh scripts/install.sh $(INSTALL_ARGS) --workflow multi-agent

uninstall:
	sh scripts/uninstall.sh $(COMMON)

check:
	sh scripts/install.sh $(INSTALL_ARGS) --check

test:
	go test ./...
	@if command -v node >/dev/null; then node scripts/check-pr-review.mjs; else echo "skipped scripts/check-pr-review.mjs: node is not on PATH"; fi
	@if [ -f scripts/check-terms.mjs ] && command -v node >/dev/null; then node scripts/check-terms.mjs; else echo "skipped scripts/check-terms.mjs: missing or node is not on PATH"; fi
