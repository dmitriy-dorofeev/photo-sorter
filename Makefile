.PHONY: build clean test snapshot build-mac-app tools-ci fmt-check ci-check

# Версия берётся из последнего git-тега или короткого хеша коммита.
# Если рабочая директория "грязная" — добавляется суффикс -dirty.
VERSION ?= $(shell git describe --tags --always --dirty)

LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

build:
	go build $(LDFLAGS) -o bin/photo-sorter ./cmd

download-models:
	bash scripts/download-face-models.sh

clean:
	rm -rf bin/ dist/

test:
	go test ./...

# Установка тех же версий анализаторов, что используются в CI.
tools-ci:
	go install honnef.co/go/tools/cmd/staticcheck@2026.1
	go install golang.org/x/vuln/cmd/govulncheck@v1.3.0
	go install github.com/securego/gosec/v2/cmd/gosec@v2.22.11

# Проверка, что в репозитории нет неотформатированных Go-файлов.
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Найдены неотформатированные файлы:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

# Локальный прогон проверок уровня CI.
ci-check: fmt-check tools-ci
	@PATH="$$(go env GOPATH)/bin:$$PATH" go vet ./...
	@PATH="$$(go env GOPATH)/bin:$$PATH" staticcheck ./...
	@PATH="$$(go env GOPATH)/bin:$$PATH" govulncheck ./...
	@PATH="$$(go env GOPATH)/bin:$$PATH" gosec -exclude=G404 ./...
	go test ./...
	go build -o photo-sorter ./cmd

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
