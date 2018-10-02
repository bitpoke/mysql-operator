package docker

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	reg "github.com/appscode/docker-registry-client/registry"
	httpz "github.com/appscode/go/net/http"
	manifestV2 "github.com/docker/distribution/manifest/schema2"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/kubernetes/pkg/credentialprovider"
	_ "k8s.io/kubernetes/pkg/credentialprovider/aws"
	_ "k8s.io/kubernetes/pkg/credentialprovider/azure"
	_ "k8s.io/kubernetes/pkg/credentialprovider/gcp"
	_ "k8s.io/kubernetes/pkg/credentialprovider/rancher"
	"k8s.io/kubernetes/pkg/credentialprovider/secrets"
	"k8s.io/kubernetes/pkg/util/parsers"
)

var (
	ErrManifestV2Required = errors.New("image manifest must of v2 format")
)

func MakeDockerKeyring(pullSecrets []v1.Secret) (credentialprovider.DockerKeyring, error) {
	return secrets.MakeDockerKeyring(pullSecrets, credentialprovider.NewDockerKeyring())
}

type ImageRef struct {
	RepoToPull  string
	Tag         string
	Digest      string
	RegistryURL string
	Repository  string
}

func (i ImageRef) Ref() string {
	if i.Digest != "" {
		return i.Digest
	}
	return i.Tag
}

func (i ImageRef) String() string {
	if i.Digest != "" {
		return i.RepoToPull + "@" + i.Digest
	}
	return i.RepoToPull + ":" + i.Tag
}

func ParseImageName(image string) (ref ImageRef, err error) {
	ref.RepoToPull, ref.Tag, ref.Digest, err = parsers.ParseImageName(image)
	if err != nil {
		return
	}

	parts := strings.SplitN(ref.RepoToPull, "/", 2)

	registryURL := parts[0]
	if strings.HasPrefix(registryURL, "docker.io") || strings.HasPrefix(registryURL, "index.docker.io") {
		registryURL = "registry-1.docker.io"
	}
	if !strings.HasPrefix(registryURL, "https://") && !strings.HasPrefix(registryURL, "http://") {
		registryURL = "https://" + registryURL
	}
	ref.RegistryURL = registryURL
	ref.Repository = parts[1]

	_, err = url.Parse(ref.RegistryURL)
	return
}

// PullManifest pulls an image manifest (v2 or v1) from remote registry using the supplied secrets if necessary.
// ref: https://github.com/kubernetes/kubernetes/blob/release-1.9/pkg/kubelet/kuberuntime/kuberuntime_image.go#L29
func PullManifest(ref ImageRef, keyring credentialprovider.DockerKeyring) (*reg.Registry, *dockertypes.AuthConfig, interface{}, error) {
	creds, withCredentials := keyring.Lookup(ref.RepoToPull)
	if !withCredentials {
		glog.V(3).Infof("Pulling image %q without credentials", ref)
		auth := &dockertypes.AuthConfig{ServerAddress: ref.RegistryURL}
		hub, mf, err := pullManifest(ref, auth)
		return hub, auth, mf, err
	}

	var pullErrs []error
	for _, currentCreds := range creds {
		authConfig := credentialprovider.LazyProvide(currentCreds)
		auth := &dockertypes.AuthConfig{
			Username:      authConfig.Username,
			Password:      authConfig.Password,
			Auth:          authConfig.Auth,
			ServerAddress: authConfig.ServerAddress,
		}
		if auth.ServerAddress == "" {
			auth.ServerAddress = ref.RegistryURL
		}

		hub, mf, err := pullManifest(ref, auth)
		// If there was no error, return success
		if err == nil {
			return hub, auth, mf, nil
		}
		pullErrs = append(pullErrs, err)
	}
	return nil, nil, nil, utilerrors.NewAggregate(pullErrs)
}

var transport = httpz.LogTransport(http.DefaultTransport)

func pullManifest(ref ImageRef, auth *dockertypes.AuthConfig) (*reg.Registry, interface{}, error) {
	hub := &reg.Registry{
		URL: auth.ServerAddress,
		Client: &http.Client{
			Transport: reg.WrapTransport(transport, auth.ServerAddress, auth.Username, auth.Password),
		},
		Logf: reg.Log,
	}
	mf, err := hub.ManifestVx(ref.Repository, ref.Ref())
	return hub, mf, err
}

// GetLabels returns the labels of docker image. The image name should how it is presented to a Kubernetes container.
// If image is found it returns tuple {labels, err=nil}, otherwise it returns tuple {label=nil, err}
func GetLabels(hub *reg.Registry, ref ImageRef, mf interface{}) (map[string]string, error) {
	switch manifest := mf.(type) {
	case *manifestV2.DeserializedManifest:
		resp, err := hub.DownloadLayer(ref.Repository, manifest.Config.Digest)
		if err != nil {
			return nil, err
		}
		defer resp.Close()

		var cfg dockertypes.ImageInspect
		err = json.NewDecoder(resp).Decode(&cfg)
		if err != nil {
			return nil, err
		}

		result := map[string]string{}
		for k, v := range cfg.Config.Labels {
			result[k] = v
		}
		return result, nil
	}
	return nil, ErrManifestV2Required
}
