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

# ── Автоматическое создание релизных тегов ────────────────────────────────

# Определяем последний semver-тег (vX.Y.Z) или используем v0.0.0
LAST_TAG := $(shell git describe --tags --match 'v*' --abbrev=0 2>/dev/null || echo "v0.0.0")

# Парсим X.Y.Z
VERSION_MAJOR := $(shell echo $(LAST_TAG) | sed 's/v\([0-9]*\).*/\1/')
VERSION_MINOR := $(shell echo $(LAST_TAG) | sed 's/v[0-9]*\.\([0-9]*\).*/\1/')
VERSION_PATCH := $(shell echo $(LAST_TAG) | sed 's/v[0-9]*\.[0-9]*\.\([0-9]*\).*/\1/')

NEXT_PATCH := v$(VERSION_MAJOR).$(VERSION_MINOR).$(shell echo $$(( $(VERSION_PATCH) + 1 )))
NEXT_MINOR := v$(VERSION_MAJOR).$(shell echo $$(( $(VERSION_MINOR) + 1 ))).0
NEXT_MAJOR := v$(shell echo $$(( $(VERSION_MAJOR) + 1 ))).0.0

.PHONY: release-patch release-minor release-major

release-patch:
	@echo "Последний тег: $(LAST_TAG) → новый тег: $(NEXT_PATCH)"
	git tag -a $(NEXT_PATCH) -m "Release $(NEXT_PATCH)"
	git push origin $(NEXT_PATCH)

release-minor:
	@echo "Последний тег: $(LAST_TAG) → новый тег: $(NEXT_MINOR)"
	git tag -a $(NEXT_MINOR) -m "Release $(NEXT_MINOR)"
	git push origin $(NEXT_MINOR)

release-major:
	@echo "Последний тег: $(LAST_TAG) → новый тег: $(NEXT_MAJOR)"
	git tag -a $(NEXT_MAJOR) -m "Release $(NEXT_MAJOR)"
	git push origin $(NEXT_MAJOR)
