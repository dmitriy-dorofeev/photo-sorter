.PHONY: build clean test snapshot build-mac-app

# Версия берётся из последнего git-тега или короткого хеша коммита.
# Если рабочая директория "грязная" — добавляется суффикс -dirty.
VERSION ?= $(shell git describe --tags --always --dirty)

LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

build:
	go build $(LDFLAGS) -o bin/photo-sorter ./cmd

clean:
	rm -rf bin/ dist/

test:
	go test ./...

# Локальная сборка "снимка" (snapshot) через GoReleaser без публикации релиза.
# Требует установленного goreleaser: https://goreleaser.com/install/
build-mac-app: build
	@echo "Сборка .app bundle для macOS..."
	@bash scripts/package-macos-app.sh bin/photo-sorter bin > /dev/null
	@rm -f "bin/Photo Sorter.app.zip"
	@ditto -c -k --keepParent "bin/Photo Sorter.app" "bin/Photo Sorter.app.zip"
	@if [ -f build/macos/icon.png ]; then \
		echo "Иконка сконвертирована из build/macos/icon.png"; \
	else \
		echo "⚠️  Иконка не найдена. Положите build/macos/icon.png (1024×1024) и пересоберите."; \
	fi
	@echo "Готово: bin/Photo Sorter.app"
	@echo "Архив:  bin/Photo Sorter.app.zip"

snapshot:
	goreleaser release --snapshot --clean --skip=publish
