package vipdoc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quantbeing/tdx/model"
)

type Reader struct {
	root string
}

func Open(root string) (*Reader, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("%w: empty root path", ErrInvalidRoot)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("%w: path=%s: %v", ErrInvalidRoot, root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: path=%s is not a directory", ErrInvalidRoot, root)
	}
	return &Reader{root: root}, nil
}

func (r *Reader) Root() string {
	return r.root
}

func (r *Reader) dailyPath(symbol model.Symbol) (string, error) {
	market, err := marketDir(symbol.Market)
	if err != nil {
		return "", err
	}
	code, err := symbolFileCode(symbol.Code)
	if err != nil {
		return "", err
	}
	return filepath.Join(r.root, "vipdoc", market, "lday", market+code+".day"), nil
}

func (r *Reader) blockPath(filename string) (string, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "", fmt.Errorf("%w: empty block filename", ErrInvalidInput)
	}
	if filepath.IsAbs(filename) {
		return "", fmt.Errorf("%w: block filename=%s is an absolute path", ErrInvalidInput, filename)
	}
	clean := filepath.Clean(filename)
	if clean == "." {
		return "", fmt.Errorf("%w: block filename=%s is not a file", ErrInvalidInput, filename)
	}
	var path string
	if clean != filepath.Base(clean) {
		path = filepath.Join(r.root, clean)
	} else {
		path = filepath.Join(r.root, "vipdoc", "sh", "block", clean)
	}
	if err := requirePathInRoot(r.root, path); err != nil {
		return "", fmt.Errorf("%w: block filename=%s escapes root: %v", ErrInvalidInput, filename, err)
	}
	return path, nil
}

func (r *Reader) minutePath(symbol model.Symbol, period MinutePeriod) (string, error) {
	market, err := marketDir(symbol.Market)
	if err != nil {
		return "", err
	}
	code, err := symbolFileCode(symbol.Code)
	if err != nil {
		return "", err
	}
	ext := ".lc" + period.String()
	return filepath.Join(r.root, "vipdoc", market, "minline", market+code+ext), nil
}

func marketDir(market model.Market) (string, error) {
	switch market {
	case model.MarketSH:
		return "sh", nil
	case model.MarketSZ:
		return "sz", nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedMarket, market)
	}
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func symbolFileCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("%w: empty symbol code", ErrInvalidInput)
	}
	if strings.ContainsAny(code, `/\`) {
		return "", fmt.Errorf("%w: invalid symbol code=%s: contains path separator", ErrInvalidInput, code)
	}
	if strings.Contains(code, ".") {
		return "", fmt.Errorf("%w: invalid symbol code=%s: contains dot component", ErrInvalidInput, code)
	}
	return code, nil
}

func requirePathInRoot(root string, path string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path=%s root=%s", absPath, absRoot)
	}
	return nil
}
