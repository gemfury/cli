GIT_DESCRIBE=$$(git describe --tags --always --match "v*" | sed 's/^v//')
GOLDFLAGS="-X main.Version=$(GIT_DESCRIBE)"
GO_CMD?=go

.PHONY: fury bin/linux bin/windows clean

fury: # creates the Fury CLI binaries for current platform
	$(GO_CMD) build -ldflags $(GOLDFLAGS) -o ./fury ./cmd/fury

bin/linux: # creates the Fury CLI binaries for Linux (AMD64)
	GOOS=linux GOARCH=amd64 $(GO_CMD) build -ldflags $(GOLDFLAGS) -o ./fury ./cmd/fury

bin/windows: # create windows binaries
	GOOS=windows GOARCH=amd64 $(GO_CMD) build -ldflags $(GOLDFLAGS) -o ./fury.exe ./cmd/fury

clean: # remove binary
	rm -f ./fury ./fury.exe
