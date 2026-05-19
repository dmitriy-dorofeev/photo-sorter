package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"photo-sorter/internal/updater"

	"golang.org/x/mod/semver"
)

// runUpdate выполняет self-update до последней версии с GitHub Releases.
func runUpdate() {
	res := updater.CheckVersion(version)

	if res.IsDirty {
		fmt.Println("Текущая версия собрана из 'грязного' дерева (dirty).")
		fmt.Println("Обновление невозможно: версия не соответствует релизу.")
		fmt.Println("Соберите бинарник из чистого состояния или скачайте вручную:")
		fmt.Printf("  https://github.com/%s/%s/releases/latest\n", updater.GithubOwner, updater.GithubRepo)
		os.Exit(1)
	}

	current := updater.NormalizeVersion(version)

	if res.IsDev || !semver.IsValid(current) {
		fmt.Println("Текущая версия — dev (сборка из исходников).")
		fmt.Println("Для обновления скачайте бинарник вручную:")
		fmt.Printf("  https://github.com/%s/%s/releases/latest\n", updater.GithubOwner, updater.GithubRepo)
		os.Exit(1)
	}

	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Не удалось проверить обновления: %v\n", res.Error)
		os.Exit(1)
	}

	normLatest := updater.NormalizeVersion(res.Latest)

	if semver.Compare(normLatest, current) <= 0 {
		fmt.Printf("У вас установлена актуальная версия: %s\n", version)
		os.Exit(0)
	}

	fmt.Printf("Доступно обновление: %s → %s\n", version, strings.TrimPrefix(normLatest, "v"))

	assetName := fmt.Sprintf("photo-sorter_%s_%s_%s.tar.gz",
		strings.TrimPrefix(normLatest, "v"),
		title(runtime.GOOS),
		archName(runtime.GOARCH),
	)

	latest, err := updater.FetchLatestRelease("photo-sorter/" + version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось получить информацию о релизе: %v\n", err)
		os.Exit(1)
	}

	var downloadURL, checksumsURL string
	for _, a := range latest.Assets {
		if a.Name == assetName {
			downloadURL = a.URL
		}
		if a.Name == "checksums.txt" {
			checksumsURL = a.URL
		}
	}

	if downloadURL == "" {
		fmt.Fprintf(os.Stderr, "Не найден бинарник для %s/%s (%s)\n", runtime.GOOS, runtime.GOARCH, assetName)
		fmt.Fprintf(os.Stderr, "Скачайте вручную: https://github.com/%s/%s/releases/%s\n", updater.GithubOwner, updater.GithubRepo, latest.TagName)
		os.Exit(1)
	}

	fmt.Printf("Скачивание %s...\n", assetName)
	tmpDir, err := os.MkdirTemp("", "photo-sorter-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания временной директории: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(downloadURL, archivePath); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка скачивания: %v\n", err)
		os.Exit(1)
	}

	if checksumsURL != "" {
		if err := verifyChecksum(archivePath, checksumsURL, assetName); err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка проверки целостности: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Проверка целостности пройдена.")
	} else {
		fmt.Fprintln(os.Stderr, "Предупреждение: checksums.txt не найден в релизе, проверка целостности пропущена.")
	}

	binPath, err := extractBinary(archivePath, tmpDir, "photo-sorter")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка распаковки: %v\n", err)
		os.Exit(1)
	}

	if err := replaceExecutable(binPath); err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось заменить бинарник: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Обновление до %s завершено успешно.\n", strings.TrimPrefix(normLatest, "v"))
}

// runCheckUpdate только сообщает, есть ли новая версия.
func runCheckUpdate() {
	res := updater.CheckVersion(version)

	if res.IsDirty {
		fmt.Println("Текущая версия собрана из 'грязного' дерева (dirty).")
		fmt.Println("Проверка обновлений невозможна: версия не соответствует релизу.")
		fmt.Printf("Последняя стабильная версия: https://github.com/%s/%s/releases/latest\n", updater.GithubOwner, updater.GithubRepo)
		os.Exit(1)
	}

	if res.IsDev {
		fmt.Println("Текущая версия — dev.")
		fmt.Printf("Последняя стабильная версия: https://github.com/%s/%s/releases/latest\n", updater.GithubOwner, updater.GithubRepo)
		return
	}

	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Не удалось проверить обновления: %v\n", res.Error)
		os.Exit(1)
	}

	if !res.HasUpdate {
		fmt.Printf("У вас актуальная версия: %s\n", version)
	} else {
		fmt.Printf("Доступно обновление: %s → %s\n", version, res.Latest)
		fmt.Printf("Выполните \"photo-sorter update\" для установки.\n")
	}
}

// verifyChecksum скачивает checksums.txt, парсит хеш для assetName и сверяет с SHA256 архива.
func verifyChecksum(archivePath, checksumsURL, assetName string) error {
	checksumsData, err := downloadBytes(checksumsURL)
	if err != nil {
		return fmt.Errorf("скачивание checksums.txt: %w", err)
	}

	archiveHash, err := fileSHA256(archivePath)
	if err != nil {
		return fmt.Errorf("вычисление SHA256 архива: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(checksumsData))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		expectedHash := fields[0]
		fileName := fields[1]
		if fileName == assetName {
			if !strings.EqualFold(expectedHash, archiveHash) {
				return fmt.Errorf("SHA256 mismatch: ожидалось %s, получено %s", expectedHash, archiveHash)
			}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("чтение checksums.txt: %w", err)
	}
	return fmt.Errorf("файл %s не найден в checksums.txt", assetName)
}

func fileSHA256(path string) (string, error) {
	// #nosec G304 — путь передан из доверенного источника (временный файл загрузки).
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func downloadBytes(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func downloadFile(url, path string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	// #nosec G304 — путь — временный файл в контролируемой директории.
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(archivePath, destDir, binName string) (string, error) {
	// #nosec G304 — путь — временный файл, созданный приложением.
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if filepath.Base(hdr.Name) == binName {
			outPath := filepath.Join(destDir, binName)
			mode := os.FileMode(0o755)
			if hdr.Mode >= 0 && hdr.Mode <= 0o7777 {
				mode = os.FileMode(uint32(hdr.Mode))&os.ModePerm | 0o111
			}
			// #nosec G304 — путь формируется из destDir (временная директория) и контролируемого имени бинарника.
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return "", err
			}
			const maxExtractSize = 500 << 20 // 500 MB
			_, copyErr := io.Copy(out, io.LimitReader(tr, maxExtractSize))
			if err := out.Close(); err != nil {
				return "", fmt.Errorf("закрытие файла: %w", err)
			}
			if copyErr != nil {
				return "", err
			}
			return outPath, nil
		}
	}

	return "", fmt.Errorf("бинарник %s не найден в архиве", binName)
}

func replaceExecutable(newBin string) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return err
	}

	backupPath := execPath + ".bak"
	removeSilent(backupPath)
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("бэкап: %w", err)
	}

	// Копируем новый бинарник во временный файл в той же директории,
	// чтобы rename не упал с EXDEV при cross-device обновлении (например, /tmp на tmpfs).
	execDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(execDir, filepath.Base(execPath)+".tmp.")
	if err != nil {
		renameSilent(backupPath, execPath)
		return fmt.Errorf("создание временного файла: %w", err)
	}
	tmpPath := tmpFile.Name()

	// #nosec G304 — путь — временный файл, извлечённый из доверенного архива.
	src, err := os.Open(newBin)
	if err != nil {
		closeSilent(tmpFile)
		removeSilent(tmpPath)
		renameSilent(backupPath, execPath)
		return fmt.Errorf("открытие нового бинарника: %w", err)
	}
	defer src.Close()

	if _, err := io.Copy(tmpFile, src); err != nil {
		closeSilent(tmpFile)
		removeSilent(tmpPath)
		renameSilent(backupPath, execPath)
		return fmt.Errorf("копирование: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		removeSilent(tmpPath)
		renameSilent(backupPath, execPath)
		return fmt.Errorf("закрытие временного файла: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		removeSilent(tmpPath)
		renameSilent(backupPath, execPath)
		return fmt.Errorf("замена: %w", err)
	}

	// #nosec G302 — исполняемый бинарник требует прав на запуск.
	if err := os.Chmod(execPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Предупреждение: не удалось установить права: %v\n", err)
	}

	removeSilent(backupPath)
	return nil
}

func title(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func archName(a string) string {
	if a == "amd64" {
		return "x86_64"
	}
	return a
}

// cleanup helpers — ignore errors on best-effort recovery operations.

func removeSilent(path string) {
	if err := os.Remove(path); err != nil {
		// cleanup best-effort
	}
}

func renameSilent(oldpath, newpath string) {
	if err := os.Rename(oldpath, newpath); err != nil {
		// cleanup best-effort
	}
}

func closeSilent(c io.Closer) {
	if err := c.Close(); err != nil {
		// cleanup best-effort
	}
}
