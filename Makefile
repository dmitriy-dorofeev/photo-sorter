.PHONY: build clean test snapshot

# Версия берётся из последнего git-тега или короткого хеша коммита.
# Если рабочая директория "грязная" — добавляется суффикс -dirty.
VERSION ?= $(shell git describe --tags --always --dirty)

LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

build:
	go build $(LDFLAGS) -o bin/photo-sorter cmd/main.go

clean:
	rm -rf bin/ dist/

test:
	go test ./...

# Локальная сборка "снимка" (snapshot) через GoReleaser без публикации релиза.
# Требует установленного goreleaser: https://goreleaser.com/install/
snapshot:
	goreleaser release --snapshot --clean --skip=publish
