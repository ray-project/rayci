package wanda

import (
	"fmt"
	"log"
	"net/http/httptest"
	"testing"

	cranename "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestResolveLocalImage(t *testing.T) {
	random, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal("create random image: ", err)
	}

	const tagStr = "cr.ray.io/rayproject/wanda:resolve-test"
	tag, err := cranename.NewTag(tagStr)
	if err != nil {
		t.Fatal("prase tag: ", err)
	}

	imageID, err := random.ConfigName()
	if err != nil {
		t.Fatal("get image id: ", err)
	}

	resp, err := daemon.Write(tag, random)
	if err != nil {
		t.Fatal("save image to daemon: ", err)
	}

	log.Println("image id: ", resp)

	src, err := resolveLocalImage("test-img", tagStr)
	if err != nil {
		t.Fatal("resolve image: ", err)
	}

	if src.name != "test-img" {
		t.Errorf("got image name %q, want `test-img`", src.name)
	}
	if want := imageID.String(); want != src.id {
		t.Errorf("got image id %q, want %q", src.id, want)
	}

	dockerCmd := newDockerCmd("")
	if err := dockerCmd.run("image", "rm", tagStr); err != nil {
		t.Fatal("remove image: ", err)
	}
}

func TestResolveRemoteImage(t *testing.T) {
	cr := registry.New()
	server := httptest.NewServer(cr)
	defer server.Close()

	addr := server.Listener.Addr().String()

	random, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal("create random image: ", err)
	}

	tagStr := fmt.Sprintf("%s/rayproject/wanda:resolve-test", addr)
	ref, err := cranename.ParseReference(tagStr)
	if err != nil {
		t.Fatal("prase tag: ", err)
	}

	imageID, err := random.ConfigName()
	if err != nil {
		t.Fatal("get image id: ", err)
	}

	if err := remote.Write(ref, random); err != nil {
		t.Fatal("save image to remote: ", err)
	}

	src, err := resolveRemoteImage("test-img", tagStr)
	if err != nil {
		t.Fatal("resolve image: ", err)
	}

	if src.name != "test-img" {
		t.Errorf("got image name %q, want `test-img`", src.name)
	}
	if want := imageID.String(); want != src.id {
		t.Errorf("got image id %q, want %q", src.id, want)
	}
}
