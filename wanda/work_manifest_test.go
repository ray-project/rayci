package wanda

import (
	"fmt"
	"net/http/httptest"
	"testing"

	cranename "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestPutWorkManifest(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	repo := server.Listener.Addr().String() + "/work"

	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal("create random image: ", err)
	}
	cacheTag, err := cranename.NewTag(repo + ":z-cache")
	if err != nil {
		t.Fatal("parse cache tag: ", err)
	}
	if err := remote.Write(cacheTag, img); err != nil {
		t.Fatal("push cache image: ", err)
	}
	cache, err := remote.Get(cacheTag)
	if err != nil {
		t.Fatal("get cache image: ", err)
	}
	cacheImg, err := cache.Image()
	if err != nil {
		t.Fatal("read cache image: ", err)
	}
	cacheManifest, err := cacheImg.Manifest()
	if err != nil {
		t.Fatal("read cache manifest: ", err)
	}

	seen := make(map[string]struct{})
	for _, buildID := range []string{"abc123", "def456"} {
		workTag, err := cranename.NewTag(fmt.Sprintf("%s:%s-hello", repo, buildID))
		if err != nil {
			t.Fatal("parse work tag: ", err)
		}
		if err := putWorkManifest(workTag, cache, buildID); err != nil {
			t.Fatalf("putWorkManifest(%q): %v", buildID, err)
		}

		work, err := remote.Get(workTag)
		if err != nil {
			t.Fatal("get work image: ", err)
		}
		if work.Digest == cache.Digest {
			t.Errorf("build %q work digest = cache digest %s, want a new one", buildID, work.Digest)
		}
		if _, ok := seen[work.Digest.String()]; ok {
			t.Errorf("build %q reused work digest %s", buildID, work.Digest)
		}
		seen[work.Digest.String()] = struct{}{}
		if work.MediaType != cache.MediaType {
			t.Errorf("work media type = %s, want %s", work.MediaType, cache.MediaType)
		}

		workImg, err := work.Image()
		if err != nil {
			t.Fatal("read work image: ", err)
		}
		m, err := workImg.Manifest()
		if err != nil {
			t.Fatal("read work manifest: ", err)
		}
		if got := m.Annotations[buildIDAnnotation]; got != buildID {
			t.Errorf("build ID annotation = %q, want %q", got, buildID)
		}
		if m.Config.Digest != cacheManifest.Config.Digest {
			t.Errorf("config digest = %s, want %s", m.Config.Digest, cacheManifest.Config.Digest)
		}
		if len(m.Layers) != len(cacheManifest.Layers) {
			t.Fatalf("got %d layers, want %d", len(m.Layers), len(cacheManifest.Layers))
		}
		for i, l := range m.Layers {
			if want := cacheManifest.Layers[i].Digest; l.Digest != want {
				t.Errorf("layer %d digest = %s, want %s", i, l.Digest, want)
			}
		}
	}

	after, err := remote.Get(cacheTag)
	if err != nil {
		t.Fatal("get cache image after: ", err)
	}
	if after.Digest != cache.Digest {
		t.Errorf("cache digest = %s, want unchanged %s", after.Digest, cache.Digest)
	}
}

func TestPutWorkManifest_indexTaggedAsIs(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	repo := server.Listener.Addr().String() + "/work"

	idx, err := random.Index(1024, 1, 2)
	if err != nil {
		t.Fatal("create random index: ", err)
	}
	cacheTag, err := cranename.NewTag(repo + ":z-cache")
	if err != nil {
		t.Fatal("parse cache tag: ", err)
	}
	if err := remote.WriteIndex(cacheTag, idx); err != nil {
		t.Fatal("push cache index: ", err)
	}
	cache, err := remote.Get(cacheTag)
	if err != nil {
		t.Fatal("get cache index: ", err)
	}

	workTag, err := cranename.NewTag(repo + ":abc123-hello")
	if err != nil {
		t.Fatal("parse work tag: ", err)
	}
	if err := putWorkManifest(workTag, cache, "abc123"); err != nil {
		t.Fatal("putWorkManifest: ", err)
	}
	work, err := remote.Get(workTag)
	if err != nil {
		t.Fatal("get work index: ", err)
	}
	if work.Digest != cache.Digest {
		t.Errorf("work digest = %s, want cache index digest %s", work.Digest, cache.Digest)
	}
}
