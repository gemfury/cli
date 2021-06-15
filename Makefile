GIT_DESCRIBE=$$(git describe --tags --always --match "v*")
GOLDFLAGS="-X main.GitDescribe=$(GIT_DESCRIBE)"
GO_CMD?=go

bin: # creates the Fury CLI binaries for current platform
	$(GO_CMD) build -ldflags $(GOLDFLAGS) -o ./fury ./cmd/fury

bin/linux: # creates the Fury CLI binaries for Linux (AMD64)
	GOOS=linux GOARCH=amd64 $(GO_CMD) build -ldflags $(GOLDFLAGS) -o ./fury ./cmd/fury

bin/windows: # create windows binaries
	GOOS=windows GOARCH=amd64 $(GO_CMD) build -ldflags $(GOLDFLAGS) -o ./fury.exe ./cmd/fury
