GO ?= $(shell which go)
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)

IMAGE_NAME := "webhook"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

KUBE_VERSION=1.26.0

$(shell mkdir -p "$(OUT)")
export TEST_ASSET_ETCD=_test/kubebuilder/etcd
export TEST_ASSET_KUBE_APISERVER=_test/kubebuilder/kube-apiserver
export TEST_ASSET_KUBECTL=_test/kubebuilder/kubectl

test: _test/kubebuilder
	$(GO) test -v .

_test/kubebuilder:
	curl -fsSL https://go.kubebuilder.io/test-tools/$(KUBE_VERSION)/$(OS)/$(ARCH) -o kubebuilder-tools.tar.gz
	mkdir -p _test/kubebuilder
	tar -xvf kubebuilder-tools.tar.gz
	mv kubebuilder/bin/* _test/kubebuilder/
	rm kubebuilder-tools.tar.gz
	rm -R kubebuilder
	mkdir -p testdata/googledomains

clean: clean-kubebuilder

clean-kubebuilder:
	rm -Rf _test/kubebuilder
	rm -Rf testdata/googledomains

REGISTRY = "nblxa.github.io"
IMAGE_NAME = "cert-manager-webhook-google-domains"
IMAGE_TAG  = "0.1.0"

build:
	DOCKER_BUILDKIT=1 docker build -t "$(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)" .

push-release:
	docker push "$(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)"

build-latest:
	DOCKER_BUILDKIT=1 docker build -t "$(REGISTRY)/$(IMAGE_NAME):latest" .

push-latest:
	docker push "$(REGISTRY)/$(IMAGE_NAME):latest"

verify-chart:
	helm lint chart/

generate-chart:
	helm package chart/
	helm repo index --url https://nblxa.github.io/charts .
	mkdir -p _chart/
	mv cert-manager-webhook-google-domains-*.tgz _chart/
	mv index.yaml _chart/
