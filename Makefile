PKG = github.com/QsanJohnson/csi-file-qsan
GOOS ?= linux
GOARCH ?= "amd64"
REGISTRY_NAME=johnsoncheng0210
IMAGE_NAME=qsan-file-csi
IMAGE_VERSION?=v1.0.0
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -X ${PKG}/pkg/nfs.driverVersion=${IMAGE_VERSION} -X ${PKG}/pkg/nfs.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/nfs.buildDate=${BUILD_DATE}
EXT_LDFLAGS = -s -w -extldflags "-static"
IMAGE_TAG=$(REGISTRY_NAME)/$(IMAGE_NAME):$(IMAGE_VERSION)

all: csi-driver

csi-driver:
	@mkdir -p bin
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -v -ldflags "${LDFLAGS} ${EXT_LDFLAGS}" -mod vendor -o ./bin/qsan-file-csi ./

docker:
	docker build -f Dockerfile -t $(IMAGE_TAG) .

clean:
	-rm -rf ./bin

.PHONY: all clean csi-driver docker