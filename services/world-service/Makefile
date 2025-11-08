# Shared service Makefile template

SERVICE_NAME ?= world-service
SERVICE_ROOT := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

include ../../templates/service.mk

# Example extension points:
# SERVICE_PRE_BUILD = @echo "Running pre-build tasks for $(SERVICE_NAME)"
# SERVICE_TEST_FLAGS = -run TestSmoke

