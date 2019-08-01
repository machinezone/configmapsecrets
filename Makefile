# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
#
# This file is heavily modified, but based on
# https://github.com/thockin/go-build-template


# The binary to build (just the basename).
BIN ?= configmapsecret-manager

# This repo's root import path.
REPO := github.com/machinezone/configmapsecrets
VERSION := $(shell git describe --tags --always --long --dirty --abbrev=12)
REVISION := $(shell git rev-parse --verify HEAD)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

# Where to push the docker image.
# REGISTRY ?= quay.io/machinezone
# REGISTRY ?= registry.hub.docker.com/mzinc
REGISTRY ?= mzinc


# Directories which hold non-vendored source code.
SRC_DIRS := cmd pkg

ALL_PLATFORMS := linux/amd64 linux/arm linux/arm64 # linux/ppc64le linux/s390x

# Used internally.  Users should pass GOOS and/or GOARCH.
OS := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
ARCH := $(if $(GOARCH),$(GOARCH),$(shell go env GOARCH))

GO_VERSION := $(if $(GO_VERSION),$(GO_VERSION),1.12)

BUILD_IMAGE := golang:$(GO_VERSION)-alpine
TEST_IMAGE := kubebuilder-golang-$(GO_VERSION)-alpine
BASE_IMAGE ?= gcr.io/distroless/static

dotimg = .$(1)-$(subst :,-,$(subst /,_,$(2)))

DATE ?= $(shell date +%s)


version:
	@echo $(VERSION)

clean: clean-bin clean-cache clean-docker

clean-bin:
	@rm -rf bin

clean-cache:
	@rm -rf .go

clean-docker:
	@for ID in .container-*; do [ ! -f "$$ID" ] || docker rmi -f $$(cat $$ID); done
	@rm -rf .dockerfile-* .build-* .container-* .manifest-* .pull-* .push-*

# If you want to build all binaries, see the 'all-build' rule.
# If you want to build all containers, see the 'all-container' rule.
# If you want to build AND push all containers, see the 'all-push' rule.
all: build

# For the following OS/ARCH expansions, we transform OS/ARCH into OS_ARCH
# because make pattern rules don't match with embedded '/' characters.

build-%:
	@$(MAKE) build                         \
	    --no-print-directory               \
	    GOOS=$(firstword $(subst _, ,$*))  \
	    GOARCH=$(lastword $(subst _, ,$*)) \
			DATE=$(DATE)

container-%:
	@$(MAKE) container                     \
			--no-print-directory               \
			GOOS=$(firstword $(subst _, ,$*))  \
			GOARCH=$(lastword $(subst _, ,$*)) \
			DATE=$(DATE)

push-%:
	@$(MAKE) push                         \
			--no-print-directory               \
			GOOS=$(firstword $(subst _, ,$*))  \
			GOARCH=$(lastword $(subst _, ,$*)) \
			DATE=$(DATE)

all-build: $(addprefix build-, $(subst /,_, $(ALL_PLATFORMS)))

all-container: $(addprefix container-, $(subst /,_, $(ALL_PLATFORMS)))

all-push: $(addprefix push-, $(subst /,_, $(ALL_PLATFORMS)))


###
###  TEST
###

DOT_PULL_BUILD_IMAGE = $(call dotimg,pull,$(BUILD_IMAGE))
DOT_PULL_BASE_IMAGE = $(call dotimg,pull,$(BASE_IMAGE))
DOT_BUILD_TEST_IMAGE = $(call dotimg,container,$(TEST_IMAGE))

$(DOT_PULL_BUILD_IMAGE):
	@docker pull $(BUILD_IMAGE)
	@docker images -q $(BUILD_IMAGE) > $@

$(DOT_PULL_BASE_IMAGE):
	@docker pull $(BASE_IMAGE)
	@docker images -q $(BASE_IMAGE) > $@

$(DOT_BUILD_TEST_IMAGE): $(DOT_PULL_BUILD_IMAGE) Dockerfile.test
	@sed -e "s|{ARG_FROM}|$(BUILD_IMAGE)|g" Dockerfile.test | \
		docker build -t $(TEST_IMAGE) -
	@docker images -q $(TEST_IMAGE) > $@

# Example: make shell CMD="-c 'date > datefile'"
shell: $(DOT_BUILD_TEST_IMAGE) $(BUILD_DIRS)
	@echo "launching a shell in the containerized test environment"
	@docker run                                                    \
	    -ti                                                        \
	    --rm                                                       \
	    -u $$(id -u):$$(id -g)                                     \
	    -v $$(pwd):/src                                            \
	    -w /src                                                    \
			-v $$(pwd)/.go/bin/$(OS)_$(ARCH):/go/bin                   \
	    -v $$(pwd)/.go/bin/$(OS)_$(ARCH):/go/bin/$(OS)_$(ARCH)     \
	    -v $$(pwd)/.go/cache:/.cache                               \
	    --env HTTP_PROXY=$(HTTP_PROXY)                             \
	    --env HTTPS_PROXY=$(HTTPS_PROXY)                           \
	    $(TEST_IMAGE)                                              \
	    /bin/sh $(CMD)

test: $(DOT_BUILD_TEST_IMAGE) $(BUILD_DIRS)
	@docker run                                                    \
	    -i                                                         \
	    --rm                                                       \
	    -u $$(id -u):$$(id -g)                                     \
	    -v $$(pwd):/src                                            \
	    -w /src                                                    \
			-v $$(pwd)/.go/bin/$(OS)_$(ARCH):/go/bin                   \
	    -v $$(pwd)/.go/bin/$(OS)_$(ARCH):/go/bin/$(OS)_$(ARCH)     \
	    -v $$(pwd)/.go/cache:/.cache                               \
	    --env HTTP_PROXY=$(HTTP_PROXY)                             \
	    --env HTTPS_PROXY=$(HTTPS_PROXY)                           \
	    $(TEST_IMAGE)                                              \
			/bin/sh -c "./build/test.sh $(SRC_DIRS)"


###
###  BUILD
###

# Directories that we need created to build/test.
BUILD_DIRS := bin/$(OS)_$(ARCH) \
             	.go/cache

$(BUILD_DIRS):
	@mkdir -p $@

build: bin/$(OS)_$(ARCH)/$(BIN)

bin/$(OS)_$(ARCH)/$(BIN): $(BUILD_DIRS)
	@echo "building bin/$(OS)_$(ARCH)/$(BIN)"
	@docker run                                                 \
	    -i                                                      \
	    --rm                                                    \
	    -u $$(id -u):$$(id -g)                                  \
	    -v $$(pwd):/src                                         \
	    -w /src                                                 \
	    -v $$(pwd)/bin/$(OS)_$(ARCH):/go/bin/$(OS)_$(ARCH)      \
	    -v $$(pwd)/.go/cache:/.cache                            \
	    --env HTTP_PROXY=$(HTTP_PROXY)                          \
	    --env HTTPS_PROXY=$(HTTPS_PROXY)                        \
	    $(BUILD_IMAGE)                                          \
	    /bin/sh -c "                                            \
					GOOS='$(OS)'                                        \
					GOARCH='$(ARCH)'                                    \
					BIN=$(BIN)                                          \
					REPO=$(REPO)                                        \
					VERSION=$(VERSION)                                  \
					REVISION=$(REVISION)                                \
					BRANCH=$(BRANCH)                                    \
					DATE=$(DATE)                                        \
	        ./build/build.sh                                    \
	    "


###
###  CONTAINERS
###

IMAGE := $(REGISTRY)/$(BIN):$(VERSION)
IMAGE_TAG := $(IMAGE)__$(OS)_$(ARCH)
DOT_CONTAINER_IMAGE_TAG = $(call dotimg,container,$(IMAGE_TAG))

container: $(DOT_CONTAINER_IMAGE_TAG)
$(DOT_CONTAINER_IMAGE_TAG): bin/$(OS)_$(ARCH)/$(BIN) Dockerfile.in
	@echo "building image: $(IMAGE_TAG)"
	@sed                                                        \
			-e "s|{ARG_FROM}|$(BASE_IMAGE)|g"                       \
			-e "s|{ARG_OS}|$(OS)|g"                                 \
			-e "s|{ARG_ARCH}|$(ARCH)|g"                             \
			-e "s|{ARG_BIN}|$(BIN)|g"                               \
			-e "s|{ARG_REPO}|$(REPO)|g"                             \
			-e "s|{ARG_VERSION}|$(VERSION)|g"                       \
			-e "s|{ARG_REVISION}|$(REVISION)|g"                     \
			-e "s|{ARG_BRANCH}|$(BRANCH)|g"                         \
	    Dockerfile.in > .dockerfile-$(OS)_$(ARCH)
	@docker build --platform $(OS)/$(ARCH) -t $(IMAGE_TAG) -f .dockerfile-$(OS)_$(ARCH) .
	@docker images -q $(IMAGE_TAG) > $@

DOT_PUSH_IMAGE_TAG = $(call dotimg,push,$(IMAGE_TAG))

push: $(DOT_PUSH_IMAGE_TAG)
$(DOT_PUSH_IMAGE_TAG): $(DOT_CONTAINER_IMAGE_TAG)
	@echo "pushing image: $(IMAGE_TAG)"
	@docker push $(IMAGE_TAG)
	@docker images -q $(IMAGE_TAG) > $@

CONTAINERS=$(addprefix $(IMAGE)__,$(subst /,_, $(ALL_PLATFORMS)))
DOCKER_MANIFEST=DOCKER_CLI_EXPERIMENTAL=enabled docker manifest
DOT_MANIFEST_IMAGE := $(call dotimg,manifest,$(IMAGE))

manifest: $(DOT_MANIFEST_IMAGE)
$(DOT_MANIFEST_IMAGE): all-push
	@$(DOCKER_MANIFEST) create --amend $(IMAGE) $(CONTAINERS)
	@for PLATFORM in $(ALL_PLATFORMS); do                              \
			OS=$$(echo $${PLATFORM} | cut -f1 -d/);                        \
			ARCH=$$(echo $${PLATFORM} | cut -f2 -d/);                      \
			$(DOCKER_MANIFEST) annotate $(IMAGE) $(IMAGE)__$${OS}_$${ARCH} \
					--os $${OS} --arch $${ARCH};															 \
	done
	@$(DOCKER_MANIFEST) push $(IMAGE)
	@$(DOCKER_MANIFEST) inspect $(IMAGE) > $@

# @manifest-tool                                                 \
#     push from-args                                             \
#     --platforms "$$(echo $(ALL_PLATFORMS) | sed 's/ /,/g')"    \
#     --template $(IMAGE)__OS_ARCH                               \
#     --target $(IMAGE)


###
### GENERATION
###

# Generate all
generate: generate-code generate-crds generate-rbac generate-docs

generate-code: controller-gen
	@$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./pkg/api/..."

generate-crds: controller-gen
	@$(CONTROLLER_GEN) crd:trivialVersions=true paths="./pkg/..." output:stdout > config/customresourcedefinition.yaml

generate-rbac: controller-gen
	@$(CONTROLLER_GEN) rbac:roleName=configmapsecret-manager paths="./pkg/..." output:stdout > config/clusterrole.yaml

generate-docs:
	@go run cmd/genapi/main.go pkg/api/v1alpha1 > docs/api.md

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.5
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
