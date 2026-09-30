package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultRepo        = "MohamedElashri/agrep"
	DefaultBaseAPIURL  = "https://api.github.com"
	DefaultBaseHTMLURL = "https://github.com"
	DefaultTimeout     = 45 * time.Second
)

// Release represents a GitHub release.
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	HTMLURL    string  `json:"html_url"`
	Body       string  `json:"body"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset represents an attached asset in a GitHub release.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// Client interacts with GitHub releases.
type Client struct {
	HTTPClient  *http.Client
	BaseAPIURL  string
	BaseHTMLURL string
	Repo        string
	Token       string
	UserAgent   string
}

// NewClient creates a new GitHub client with sensible defaults.
func NewClient(currentVersion string) *Client {
	repo := os.Getenv("AGREP_REPOSITORY")
	if repo == "" {
		repo = DefaultRepo
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}

	ua := fmt.Sprintf("agrep/%s (%s; %s)", currentVersion, runtime.GOOS, runtime.GOARCH)

	return &Client{
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		BaseAPIURL:  DefaultBaseAPIURL,
		BaseHTMLURL: DefaultBaseHTMLURL,
		Repo:        repo,
		Token:       token,
		UserAgent:   ua,
	}
}

// LatestRelease retrieves the latest release, attempting the GitHub API first and
// falling back to GitHub HTML release redirects if rate-limited.
func (c *Client) LatestRelease(ctx context.Context) (*Release, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(c.BaseAPIURL, "/"), c.Repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var rel Release
			if err := json.NewDecoder(resp.Body).Decode(&rel); err == nil && rel.TagName != "" {
				return &rel, nil
			}
		} else if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			// Rate limited; attempt fallback
			return c.latestReleaseFallback(ctx)
		} else if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("no releases found for repository %s", c.Repo)
		}
	}

	// Try HTML fallback on network/API failure
	rel, fallbackErr := c.latestReleaseFallback(ctx)
	if fallbackErr == nil {
		return rel, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w (fallback: %v)", err, fallbackErr)
	}
	return nil, fmt.Errorf("GitHub API returned status %s (fallback: %v)", resp.Status, fallbackErr)
}

// latestReleaseFallback resolves the latest release tag via GitHub's /releases/latest 302 redirect.
func (c *Client) latestReleaseFallback(ctx context.Context) (*Release, error) {
	latestURL := fmt.Sprintf("%s/%s/releases/latest", strings.TrimRight(c.BaseHTMLURL, "/"), c.Repo)

	// Don't follow redirects so we can inspect Location header
	noRedirectClient := &http.Client{
		Timeout: c.HTTPClient.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, latestURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallback request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently && resp.StatusCode != http.StatusSeeOther {
		return nil, fmt.Errorf("expected redirect from %s, got HTTP %d", latestURL, resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return nil, errors.New("empty Location header from release redirect")
	}

	parsedLoc, err := url.Parse(loc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redirect location: %w", err)
	}

	tag := path.Base(parsedLoc.Path)
	if tag == "" || tag == "." || tag == "/" || tag == "latest" {
		return nil, fmt.Errorf("could not extract tag from redirect location: %s", loc)
	}

	return &Release{
		TagName: tag,
		Name:    tag,
		HTMLURL: loc,
	}, nil
}

// ReleaseByTag retrieves a specific release by its tag name.
func (c *Client) ReleaseByTag(ctx context.Context, tag string) (*Release, error) {
	tag = NormalizeTag(tag)
	apiURL := fmt.Sprintf("%s/repos/%s/releases/tags/%s", strings.TrimRight(c.BaseAPIURL, "/"), c.Repo, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var rel Release
			if err := json.NewDecoder(resp.Body).Decode(&rel); err == nil && rel.TagName != "" {
				return &rel, nil
			}
		} else if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("release %s not found in repository %s", tag, c.Repo)
		}
	}

	// Fallback to synthetic release for known tag
	return &Release{
		TagName: tag,
		Name:    tag,
		HTMLURL: fmt.Sprintf("%s/%s/releases/tag/%s", strings.TrimRight(c.BaseHTMLURL, "/"), c.Repo, tag),
	}, nil
}

// FindAsset locates the binary archive asset for the given OS and architecture.
func (c *Client) FindAsset(rel *Release, goos, goarch string) (*Asset, error) {
	cleanVer := CleanVersion(rel.TagName)

	// Check existing assets if provided by the API
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if !strings.HasPrefix(name, "agrep") {
			continue
		}
		targetOSArch := fmt.Sprintf("_%s_%s", strings.ToLower(goos), strings.ToLower(goarch))
		if strings.Contains(name, targetOSArch) {
			if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".zip") {
				return &a, nil
			}
		}
	}

	// If no matching asset found in API assets list, or if assets was empty (e.g. fallback mode),
	// generate the standard GoReleaser asset name and URL
	ext := "tar.gz"
	if strings.ToLower(goos) == "windows" {
		ext = "zip"
	}
	tarballName := fmt.Sprintf("agrep_%s_%s_%s.%s", cleanVer, goos, goarch, ext)
	downloadURL := fmt.Sprintf("%s/%s/releases/download/%s/%s",
		strings.TrimRight(c.BaseHTMLURL, "/"), c.Repo, rel.TagName, tarballName)

	return &Asset{
		Name:        tarballName,
		DownloadURL: downloadURL,
	}, nil
}

// FindChecksums locates the checksums.txt asset for the release.
func (c *Client) FindChecksums(rel *Release) (*Asset, error) {
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if name == "checksums.txt" || strings.HasSuffix(name, "checksums.txt") || strings.HasSuffix(name, "sha256sums.txt") {
			return &a, nil
		}
	}

	// Standard GoReleaser checksum file location
	return &Asset{
		Name: "checksums.txt",
		DownloadURL: fmt.Sprintf("%s/%s/releases/download/%s/checksums.txt",
			strings.TrimRight(c.BaseHTMLURL, "/"), c.Repo, rel.TagName),
	}, nil
}

// Download fetches an asset from downloadURL and writes it to w.
func (c *Client) Download(ctx context.Context, downloadURL string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download from %s: HTTP %s", downloadURL, resp.Status)
	}

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write downloaded content: %w", err)
	}

	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
}
