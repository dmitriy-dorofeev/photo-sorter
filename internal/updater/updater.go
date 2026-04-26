// Package updater содержит логику проверки и установки обновлений с GitHub Releases.
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	GithubOwner = "dmitriy-dorofeev"
	GithubRepo  = "photo-sorter"
)

// Release описывает ответ GitHub API для latest release.
type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckResult содержит результат проверки обновления.
type CheckResult struct {
	HasUpdate bool
	Current   string
	Latest    string
	IsDev     bool
	IsDirty   bool
	Error     error
}

// CheckVersion проверяет, есть ли новая версия на GitHub Releases.
func CheckVersion(current string) CheckResult {
	res := CheckResult{Current: current}

	if isDirtyVersion(current) {
		res.IsDirty = true
		return res
	}

	normCurrent := NormalizeVersion(current)
	if normCurrent == "dev" || !semver.IsValid(normCurrent) {
		res.IsDev = true
		return res
	}

	latest, err := fetchLatestRelease()
	if err != nil {
		res.Error = err
		return res
	}

	normLatest := NormalizeVersion(latest.TagName)
	res.Latest = strings.TrimPrefix(normLatest, "v")
	res.HasUpdate = semver.Compare(normLatest, normCurrent) > 0
	return res
}

func fetchLatestRelease() (*Release, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GithubOwner, GithubRepo)
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

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// NormalizeVersion приводит версию к виду, понятному semver.
func NormalizeVersion(v string) string {
	if v == "dev" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// IsDirtyVersion возвращает true, если версия собрана из "грязного" git-дерева.
func isDirtyVersion(v string) bool {
	return strings.Contains(v, "dirty")
}
