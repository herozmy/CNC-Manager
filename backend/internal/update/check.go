package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrNoRelease 表示仓库里一个正式发布都还没有。
// 这不是故障，界面照常显示当前版本就好。
var ErrNoRelease = errors.New("仓库里还没有正式发布")

// DefaultAPIBase 是 GitHub 的公开 API 地址。
// 做成可配置的：企业内网可能自建 GitHub，测试时也可以指向一个本地假服务。
const DefaultAPIBase = "https://api.github.com"

// cacheTTL 是「最新发布」结果的缓存时长。
//
// GitHub 对未认证请求按 IP 限流（每小时 60 次），办公室里多台机器共用出口 IP
// 很容易撞上；而且发布新版本也不是分钟级的事，缓存半小时完全够用。
const cacheTTL = 30 * time.Minute

// Release 是查到的一条发布信息。
type Release struct {
	Version     string // 标签名，如 v0.04
	ReleaseURL  string // 发布页地址，供用户手工下载
	PublishedAt string
	AssetName   string // 离线安装包的附件名
	AssetURL    string
	AssetSize   int64
}

// Client 查询 GitHub 上的最新正式发布。
type Client struct {
	apiBase string
	repo    string // owner/name
	ua      string // User-Agent，GitHub 要求必须带
	http    *http.Client

	mu       sync.Mutex
	cached   *Release
	cachedAt time.Time
}

// NewClient 构造查询客户端。apiBase 传空则用 GitHub 官方地址。
func NewClient(apiBase, repo, version string) *Client {
	if strings.TrimSpace(apiBase) == "" {
		apiBase = DefaultAPIBase
	}
	return &Client{
		apiBase: strings.TrimRight(apiBase, "/"),
		repo:    repo,
		ua:      "cnccool/" + strings.TrimPrefix(version, "v"),
		// 超时必须设：离线车间里这个请求会一直挂着，
		// 不设超时的话点一次「检查更新」要等到天荒地老。
		http: &http.Client{Timeout: 8 * time.Second},
	}
}

// Enabled 表示有没有配置仓库地址。没配就整个跳过检查。
func (c *Client) Enabled() bool { return c != nil && c.repo != "" }

// ghRelease 是 GitHub Release 接口里我们关心的字段。
type ghRelease struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// Latest 取最新正式发布。命中缓存时不会发请求。
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	if !c.Enabled() {
		return nil, errors.New("没有配置更新仓库")
	}

	c.mu.Lock()
	if c.cached != nil && time.Since(c.cachedAt) < cacheTTL {
		rel := *c.cached
		c.mu.Unlock()
		return &rel, nil
	}
	c.mu.Unlock()

	rel, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cached = rel
	c.cachedAt = time.Now()
	c.mu.Unlock()

	return rel, nil
}

// Invalidate 丢掉缓存。手工检查更新时用它，免得用户点了半天还是半小时前的结论。
func (c *Client) Invalidate() {
	c.mu.Lock()
	c.cached = nil
	c.mu.Unlock()
}

func (c *Client) fetch(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.apiBase, c.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.ua)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoRelease
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("GitHub 拒绝了请求（HTTP %d），通常是查询太频繁，过一会儿再试", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查询 GitHub 失败：HTTP %d", resp.StatusCode)
	}

	var gh ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		return nil, fmt.Errorf("解析 GitHub 返回的内容失败: %w", err)
	}
	// /releases/latest 按定义就不返回草稿和预发布，这里再挡一道，防止行为变化
	if gh.Draft || gh.Prerelease {
		return nil, ErrNoRelease
	}
	if strings.TrimSpace(gh.TagName) == "" {
		return nil, ErrNoRelease
	}

	rel := &Release{
		Version:     strings.TrimSpace(gh.TagName),
		ReleaseURL:  gh.HTMLURL,
		PublishedAt: gh.PublishedAt,
	}
	rel.AssetName, rel.AssetURL, rel.AssetSize = pickAsset(gh.Assets)
	return rel, nil
}

// pickAsset 从附件里挑出 Windows 免安装包。
//
// 优先认名字里带 windows-amd64 的；退而求其次挑第一个 .zip，
// 这样将来改了命名规则也不会立刻失效。
func pickAsset(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}) (name, url string, size int64) {
	var fallbackName, fallbackURL string
	var fallbackSize int64

	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if !strings.HasSuffix(lower, ".zip") {
			continue
		}
		if strings.Contains(lower, "windows-amd64") {
			return a.Name, a.BrowserDownloadURL, a.Size
		}
		if fallbackName == "" {
			fallbackName, fallbackURL, fallbackSize = a.Name, a.BrowserDownloadURL, a.Size
		}
	}
	return fallbackName, fallbackURL, fallbackSize
}

// Status 是给界面看的「有没有新版本」。
type Status struct {
	Current     string `json:"current"`
	Latest      string `json:"latest,omitempty"`
	HasUpdate   bool   `json:"hasUpdate"`
	ReleaseURL  string `json:"releaseUrl,omitempty"`
	PublishedAt string `json:"publishedAt,omitempty"`
	AssetName   string `json:"assetName,omitempty"`
	AssetURL    string `json:"assetUrl,omitempty"`
	AssetSize   int64  `json:"assetSize,omitempty"`
	CheckedAt   string `json:"checkedAt"`
	// 检查失败时的原因。界面可以选择不显示——没网是常态，不该报成错误。
	Error string `json:"error,omitempty"`
}

// Check 汇总一次版本检查的结果。
//
// 约定：无论成功失败都返回一个 Status，失败时把原因放进 Error。
// 调用方不需要区分「查不到」和「没有新版本」——两者对用户都是「什么都不用做」。
func (c *Client) Check(ctx context.Context, current string, fresh bool) Status {
	st := Status{Current: current, CheckedAt: time.Now().UTC().Format(time.RFC3339)}

	if !c.Enabled() {
		st.Error = "没有配置更新仓库（CNC_UPDATE_REPO）"
		return st
	}
	if fresh {
		c.Invalidate()
	}

	rel, err := c.Latest(ctx)
	if err != nil {
		st.Error = err.Error()
		return st
	}

	st.Latest = rel.Version
	st.ReleaseURL = rel.ReleaseURL
	st.PublishedAt = rel.PublishedAt
	st.AssetName = rel.AssetName
	st.AssetURL = rel.AssetURL
	st.AssetSize = rel.AssetSize
	st.HasUpdate = CompareVersion(rel.Version, current) > 0
	return st
}
