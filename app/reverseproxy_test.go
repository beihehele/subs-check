package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/beihehele/subs-check/config"
	"github.com/gin-gonic/gin"
)

// newProxyTestRouter builds the UI with an external base path. The value is
// restored after the test, and newTestRouter's own cleanup restores the rest of
// the config.
func newProxyTestRouter(t *testing.T, basePath string) *gin.Engine {
	t.Helper()
	old := config.GlobalConfig.WebBasePath
	config.GlobalConfig.WebBasePath = basePath
	t.Cleanup(func() { config.GlobalConfig.WebBasePath = old })
	return newTestRouter(t)
}

// TestRouter_ReverseProxySubpathServesAssets simulates a prefix-stripping proxy
// (location /subs-check/ { proxy_pass http://app/; }) and checks that the
// relative asset URLs in the rendered page resolve back through the proxy.
func TestRouter_ReverseProxySubpathServesAssets(t *testing.T) {
	router := newProxyTestRouter(t, "")
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := r.Clone(r.Context())
		req.URL.Path = strings.TrimPrefix(r.URL.Path, "/subs-check")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		router.ServeHTTP(w, req)
	})

	pageURL, err := url.Parse("http://example.com/subs-check/admin")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, httptest.NewRequest(http.MethodGet, pageURL.String(), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("admin page status = %d", w.Code)
	}

	for _, rel := range []string{"./static/css/app.css", "./static/bootstrap/bootstrap.min.css"} {
		if !strings.Contains(w.Body.String(), rel) {
			t.Fatalf("admin page missing %q", rel)
		}
		target := pageURL.ResolveReference(&url.URL{Path: rel})
		asset := httptest.NewRecorder()
		proxy.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, target.String(), nil))
		if asset.Code != http.StatusOK {
			t.Errorf("asset %s resolves to %s and returned %d", rel, target, asset.Code)
		}
	}
}

// TestRouter_ConfiguredWebBasePath covers subpath mounts where the proxy either
// preserves or strips the prefix. Absolute URLs plus the duplicated prefixed
// mount keep both working.
func TestRouter_ConfiguredWebBasePath(t *testing.T) {
	router := newProxyTestRouter(t, "/subs-check")

	t.Run("preserved prefix", func(t *testing.T) {
		w := doRequest(router, http.MethodGet, "/subs-check/admin", false)
		if w.Code != http.StatusOK {
			t.Fatalf("admin page status = %d", w.Code)
		}
		for _, asset := range []string{
			"/subs-check/static/css/app.css",
			"/subs-check/static/bootstrap/bootstrap.min.css",
		} {
			if !strings.Contains(w.Body.String(), asset) {
				t.Fatalf("admin page missing %q", asset)
			}
			if got := doRequest(router, http.MethodGet, asset, false); got.Code != http.StatusOK {
				t.Errorf("asset %s status = %d", asset, got.Code)
			}
		}
		if got := doRequest(router, http.MethodGet, "/subs-check/api/results", true); got.Code != http.StatusOK {
			t.Errorf("prefixed API status = %d", got.Code)
		}
		if got := doRequest(router, http.MethodGet, "/subs-check/admin/", false); got.Code != http.StatusOK {
			t.Errorf("trailing slash admin status = %d", got.Code)
		}
		if got := doRequest(router, http.MethodGet, "/subs-check/admin/results/", false); got.Code != http.StatusOK {
			t.Errorf("trailing slash results status = %d", got.Code)
		}
	})

	t.Run("stripped prefix", func(t *testing.T) {
		proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := r.Clone(r.Context())
			req.URL.Path = strings.TrimPrefix(r.URL.Path, "/subs-check")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			router.ServeHTTP(w, req)
		})
		pageURL, err := url.Parse("http://example.com/subs-check/admin")
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, httptest.NewRequest(http.MethodGet, pageURL.String(), nil))
		if w.Code != http.StatusOK {
			t.Fatalf("admin page status = %d", w.Code)
		}
		asset := "/subs-check/static/css/app.css"
		if !strings.Contains(w.Body.String(), asset) {
			t.Fatalf("admin page missing %q", asset)
		}
		got := httptest.NewRecorder()
		proxy.ServeHTTP(got, httptest.NewRequest(http.MethodGet, "http://example.com"+asset, nil))
		if got.Code != http.StatusOK {
			t.Errorf("asset %s status = %d", asset, got.Code)
		}
		trailing := httptest.NewRecorder()
		proxy.ServeHTTP(trailing, httptest.NewRequest(http.MethodGet, pageURL.String()+"/", nil))
		if trailing.Code != http.StatusOK {
			t.Errorf("trailing slash admin status = %d", trailing.Code)
		}
		if !strings.Contains(trailing.Body.String(), asset) {
			t.Errorf("trailing slash admin page missing %q", asset)
		}
	})
}
