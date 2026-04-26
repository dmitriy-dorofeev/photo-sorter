package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
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

	latest, err := fetchLatestReleaseRaw()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось получить информацию о релизе: %v\n", err)
		os.Exit(1)
	}

	var downloadURL string
	for _, a := range latest.Assets {
		if a.Name == assetName {
			downloadURL = a.URL
			break
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

// fetchLatestReleaseRaw получает информацию о последнем релизе через GitHub API.
func fetchLatestReleaseRaw() (*updater.Release, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", updater.GithubOwner, updater.GithubRepo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API вернул %s", resp.Status)
	}

	var rel updater.Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(archivePath, destDir, binName string) (string, error) {
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
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode|0111))
			if err != nil {
				return "", err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
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
	_ = os.Remove(backupPath)
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("бэкап: %w", err)
	}

	if err := os.Rename(newBin, execPath); err != nil {
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("замена: %w", err)
	}

	if err := os.Chmod(execPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Предупреждение: не удалось установить права: %v\n", err)
	}

	_ = os.Remove(backupPath)
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
