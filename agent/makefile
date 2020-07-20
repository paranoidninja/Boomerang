GOBUILD=go build
BINARY_NAME=boomerang-agent
BINARY_LINUX=$(BINARY_NAME)-linx64
BINARY_ARM64=$(BINARY_NAME)-armx64

GOFLAGS= -v -ldflags '-s'
GOTRIM=-gcflags -trimpath=$(PWD)


build-lin:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_LINUX) $(GOFLAGS) $(GOTRIM)

build-arm:
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BINARY_ARM64) $(GOFLAGS) $(GOTRIM)

all:
	make build-lin
	make build-arm
