ARCH     = amd64
BIN_NAME = wordlistctl
VERSION  = 1.1-beta
MAIN     = wordlistctl.go

all: freebsd macos linux # windows
.PHONY: build

build:
	go build -o bin/${BIN_NAME} ${MAIN}

run:
	go run ${MAIN}

freebsd: 
	GOOS=freebsd GOARCH=${ARCH} go build -o bin/freebsd/${BIN_NAME}-v${VERSION}
	
macos: 
	GOOS=darwin GOARCH=${ARCH} go build -o bin/darwin/${BIN_NAME}-v${VERSION}

linux: 
	GOOS=linux GOARCH=${ARCH} go build -o bin/linux/${BIN_NAME}-v${VERSION}

# windows:
#	GOOS=windows GOARCH=${ARCH} go build -o bin/${OS}/${BIN_NAME}-v${VERSION}
