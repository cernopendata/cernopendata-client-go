package updater

const (
	GitHubRepo   = "cernopendata/cernopendata-client-go"
	GitHubAPIURL = "https://api.github.com"
)

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ReleaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}
