package secretprovider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type FileProvider struct {
	scheme       string
	root         string
	allowSymlink bool
}

func NewLocalFileProvider(root string) (*FileProvider, error) {
	return newFileProvider("local-file", root, false)
}

func NewKubernetesFileProvider(root string) (*FileProvider, error) {
	// Kubernetes projected Secret volumes use atomic symlink swaps. Resolved
	// targets are still required to remain below the configured mount root.
	return newFileProvider("k8s-secret", root, true)
}

func newFileProvider(scheme, root string, allowSymlink bool) (*FileProvider, error) {
	root = strings.TrimSpace(root)
	if root == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("%s root must be an absolute path", scheme)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve %s root: %w", scheme, err)
	}
	return &FileProvider{scheme: scheme, root: filepath.Clean(resolved), allowSymlink: allowSymlink}, nil
}

func (p *FileProvider) Scheme() string { return p.scheme }

func (p *FileProvider) Resolve(_ context.Context, reference *url.URL) ([]byte, error) {
	if reference.Host != "" && reference.Host != "localhost" {
		return nil, errors.New("file secret reference must not contain a remote host")
	}
	relative := strings.TrimPrefix(filepath.Clean("/"+reference.Path), "/")
	if relative == "." || relative == "" {
		return nil, errors.New("file secret path is required")
	}
	candidate := filepath.Join(p.root, relative)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return nil, fmt.Errorf("resolve secret path: %w", err)
	}
	if resolved != p.root && !strings.HasPrefix(resolved, p.root+string(filepath.Separator)) {
		return nil, errors.New("secret path escapes configured root")
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 && !p.allowSymlink {
		return nil, errors.New("local secret symlinks are forbidden")
	}
	resolvedInfo, err := os.Stat(resolved)
	if err != nil || !resolvedInfo.Mode().IsRegular() {
		return nil, errors.New("secret path is not a regular file")
	}
	if resolvedInfo.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("secret file must not be readable or writable by group/others")
	}
	return os.ReadFile(resolved)
}
