SHELL := /bin/bash
ENV ?= development
TF_ROOT := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
TF_ENV_DIR := $(TF_ROOT)/environments/$(ENV)
BACKEND_CONFIG ?= $(TF_ROOT)/backend.hcl

# Allow callers to pass additional arguments, e.g. PLAN_ARGS="-out=plan.out"
PLAN_ARGS ?=
APPLY_ARGS ?=
DESTROY_ARGS ?=
DRIFT_ARGS ?=

TF := terraform -chdir=$(TF_ENV_DIR)

.PHONY: init fmt validate plan apply destroy drift state-pull ensure-env

ensure-env:
	@if [ ! -d "$(TF_ENV_DIR)" ]; then \
		echo "Missing environment directory: $(TF_ENV_DIR)"; \
		echo "Create it or choose another via ENV=<name>."; \
		exit 1; \
	fi

init: ensure-env
	$(TF) init -backend-config=$(BACKEND_CONFIG)

fmt: ensure-env
	$(TF) fmt -recursive

validate: ensure-env fmt
	$(TF) validate

plan: ensure-env fmt validate
	$(TF) plan $(PLAN_ARGS)

apply: ensure-env
	$(TF) apply $(APPLY_ARGS)

destroy: ensure-env
	$(TF) destroy $(DESTROY_ARGS)

drift: ensure-env
	$(TF) plan -refresh-only $(DRIFT_ARGS)

state-pull: ensure-env
	$(TF) state pull > $(TF_ENV_DIR)/terraform.tfstate
