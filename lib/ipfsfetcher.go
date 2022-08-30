package lib

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	api "github.com/ipfs/go-ipfs-api"
	"github.com/ipfs/ipfs-update/util"
	"github.com/ipfs/kubo/repo/fsrepo/migrations"
)

const (
	shellUpTimeout    = 2 * time.Second
	defaultFetchLimit = 1024 * 1024 * 512
)

type IpfsFetcher struct {
	distPath string
	limit    int64
}

// NewIpfsFetcher creates a new IpfsFetcher
//
// Specifying "" for distPath sets the default IPNS path.
// Specifying 0 for fetchLimit sets the default, -1 means no limit.
func NewIpfsFetcher(distPath string, fetchLimit int64) *IpfsFetcher {
	f := &IpfsFetcher{
		limit:    defaultFetchLimit,
		distPath: migrations.LatestIpfsDist,
	}

	if distPath != "" {
		if !strings.HasPrefix(distPath, "/") {
			distPath = "/" + distPath
		}
		f.distPath = distPath
	}

	if fetchLimit != 0 {
		if fetchLimit == -1 {
			fetchLimit = 0
		}
		f.limit = fetchLimit
	}

	return f
}

// Fetch attempts to fetch the file at the given path, from the distribution
// site configured for this HttpFetcher.  Returns io.ReadCloser on success,
// which caller must close.
func (f *IpfsFetcher) Fetch(ctx context.Context, filePath string) ([]byte, error) {
	sh, _, err := ApiShell("")
	if err != nil {
		return nil, err
	}

	resp, err := sh.Request("cat", path.Join(f.distPath, filePath)).Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	var rc io.ReadCloser
	if f.limit != 0 {
		rc = migrations.NewLimitReadCloser(resp.Output, f.limit)
	} else {
		rc = resp.Output
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// ApiShell creates a new ipfs api shell and checks that it is up.  If the shell
// is available, then the shell and ipfs version are returned.
func ApiShell(ipfsDir string) (*api.Shell, string, error) {
	apiEp, err := util.ApiEndpoint("")
	if err != nil {
		return nil, "", err
	}
	sh := api.NewShell(apiEp)
	sh.SetTimeout(shellUpTimeout)
	ver, _, err := sh.Version()
	if err != nil {
		return nil, "", errors.New("ipfs api shell not up")
	}
	sh.SetTimeout(0)
	return sh, ver, nil
}

func (f *IpfsFetcher) Close() error {
	return nil
}
