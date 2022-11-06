package nextexport

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
)

// can be minfs or embed.FS
type FS interface {
	Open(name string) (fs.File, error)
	ReadDir(name string) ([]fs.DirEntry, error)
}

// NOTE: pageRoute is a partial representation of static/dymamic route from
// next's route-manifest.json file.
type pageRoute struct {
	path    string
	regexp  *regexp.Regexp
	dynamic uint8
}

func getPageRoute(path string) (*pageRoute, error) {
	result := pageRoute{path: path}

	b := strings.Builder{}
	b.WriteString("^")

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, part := range parts {
		part = strings.TrimSuffix(part, ".html")

		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			b.WriteString("/([^/]+?)")
			result.dynamic = 1
			continue
		}

		b.WriteString("/" + part)
	}

	b.WriteString("(?:/)?$")

	var err error
	result.regexp, err = regexp.Compile(b.String())
	if err != nil {
		return nil, fmt.Errorf("could not compile regexp: %w", err)
	}

	return &result, nil
}

func getPageRoutes(fs FS, rootDir string, dir string) ([]*pageRoute, error) {
	dirEntries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	pageRoutes := make([]*pageRoute, 0)

	for _, dirEntry := range dirEntries {
		absolutePath := path.Join(dir, dirEntry.Name())

		if dirEntry.IsDir() {
			dirPaths, err := getPageRoutes(fs, rootDir, absolutePath)
			if err != nil {
				return nil, err
			}
			pageRoutes = append(pageRoutes, dirPaths...)
			continue
		}

		if !strings.HasSuffix(absolutePath, ".html") {
			continue
		}

		path := absolutePath
		if rootDir != "." {
			path = strings.Replace(absolutePath, rootDir, "", 1)
		}

		pr, err := getPageRoute(path)
		if err != nil {
			return nil, fmt.Errorf("could not get page route: %w", err)
		}

		pageRoutes = append(pageRoutes, pr)
	}

	// static should get a priority over dynamic
	sort.Slice(pageRoutes, func(i, j int) bool {
		return pageRoutes[i].dynamic < pageRoutes[j].dynamic
	})

	return pageRoutes, nil
}

type handler struct {
	pageRoutes []*pageRoute
	fs         FS
	rootDir    string
}

func NewHandler(fs FS, rootDir string) (http.Handler, error) {
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
