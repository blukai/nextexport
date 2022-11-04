package nextexport

import (
	"embed"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
)

// NOTE: pageRoute is a partial representation of static/dymamic route from
// next's route-manifest.json file.
type pageRoute struct {
	path   string
	regexp *regexp.Regexp
}

func getPageRouteRegexp(path string) (*regexp.Regexp, error) {
	b := strings.Builder{}
	b.WriteString("^")

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, part := range parts {
		part = strings.TrimSuffix(part, ".html")

		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			b.WriteString("/([^/]+?)")
			continue
		}

		b.WriteString("/" + part)
	}

	b.WriteString("(?:/)?$")

	return regexp.Compile(b.String())
}

func getPageRoutes(fs embed.FS, rootDir string, dir string) ([]*pageRoute, error) {
	dirEntries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	paths := make([]*pageRoute, 0)

	for _, dirEntry := range dirEntries {
		absolutePath := path.Join(dir, dirEntry.Name())

		if dirEntry.IsDir() {
			dirPaths, err := getPageRoutes(fs, rootDir, absolutePath)
			if err != nil {
				return nil, err
			}
			paths = append(paths, dirPaths...)
			continue
		}

		if !strings.HasSuffix(absolutePath, ".html") {
			continue
		}

		path := absolutePath
		if rootDir != "." {
			path = strings.Replace(absolutePath, rootDir, "", 1)
		}

		regexp, err := getPageRouteRegexp(path)
		if err != nil {
			return nil, fmt.Errorf("could not get page route regexp: %w", err)
		}

		paths = append(paths, &pageRoute{
			path:   path,
			regexp: regexp,
		})
	}

	return paths, nil
}

type handler struct {
	pageRoutes []*pageRoute
	fs         embed.FS
	rootDir    string
}

func NewHandler(fs embed.FS, rootDir string) (http.Handler, error) {
	pageRoutes, err := getPageRoutes(fs, rootDir, rootDir)
	if err != nil {
		return nil, fmt.Errorf("could not get page routes: %w", err)
	}

	return &handler{
		pageRoutes: pageRoutes,
		fs:         fs,
		rootDir:    rootDir,
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.pageRoutes {
		if route.regexp.MatchString(r.URL.Path) {
			r.URL.Path = "/" + route.path
			break
		}
	}

	if h.rootDir != "." {
		r.URL.Path = "/" + h.rootDir + r.URL.Path
	}

	fs := http.FileServer(http.FS(h.fs))
	fs.ServeHTTP(w, r)
}
