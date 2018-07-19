package doctor

import (
	"crypto/x509"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/cert"
)

type ClusterInfo struct {
	Version               *VersionInfo          `json:"version,omitempty"`
	ClientConfig          RestConfig            `json:"clientConfig,omitempty"`
	Capabilities          Capabilities          `json:"capabilities,omitempty"`
	APIServers            APIServers            `json:"apiServers,omitempty"`
	ExtensionServerConfig ExtensionServerConfig `json:"extensionServerConfig,omitempty"`
}

type VersionInfo struct {
	Minor      string `json:"minor,omitempty"`
	Patch      string `json:"patch,omitempty"`
	GitVersion string `json:"gitVersion,omitempty"`
	GitCommit  string `json:"gitCommit,omitempty"`
	BuildDate  string `json:"buildDate,omitempty"`
	Platform   string `json:"platform,omitempty"`
}

type RestConfig struct {
	Host     string
	CAData   string `json:"caData,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
}

type Capabilities struct {
	APIVersion                 string `json:"apiVersion,omitempty"`
	AggregateAPIServer         bool   `json:"aggregateAPIServer,omitempty"`
	MutatingAdmissionWebhook   string `json:"mutatingAdmissionWebhook,omitempty"`
	ValidatingAdmissionWebhook string `json:"validatingAdmissionWebhook,omitempty"`
	PodSecurityPolicy          string `json:"podSecurityPolicy,omitempty"`
	Initializers               string `json:"initializers,omitempty"`
	CustomResourceSubresources string `json:"customResourceSubresources,omitempty"`
}

type APIServerConfig struct {
	PodName             string            `json:"podName,omitempty"`
	NodeName            string            `json:"nodeName,omitempty"`
	PodIP               string            `json:"podIP,omitempty"`
	HostIP              string            `json:"hostIP,omitempty"`
	AdmissionControl    []string          `json:"admissionControl,omitempty"`
	ClientCAData        string            `json:"clientCAData,omitempty"`
	TLSCertData         string            `json:"tlsCertData,omitempty"`
	RequestHeaderCAData string            `json:"requestHeaderCAData,omitempty"`
	AllowPrivileged     bool              `json:"allowPrivileged,omitempty"`
	AuthorizationMode   []string          `json:"authorizationMode,omitempty"`
	RuntimeConfig       FeatureList       `json:"runtimeConfig,omitempty"`
	FeatureGates        FeatureList       `json:"featureGates,omitempty"`
	KubeProxyFound      bool              `json:"kubeProxyFound,omitempty"`
	KubeProxyRunning    bool              `json:"kubeProxyRunning,omitempty"`
	ProxySettings       map[string]string `json:"proxySettings,omitempty"`
}

var (
	ErrUnknown = errors.New("unknown")
)

type FeatureList struct {
	Enabled  []string `json:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

func (f FeatureList) Status(name string) (bool, error) {
	if sets.NewString(f.Enabled...).Has(name) {
		return true, nil
	}
	if sets.NewString(f.Disabled...).Has(name) {
		return false, nil
	}
	return false, ErrUnknown
}

type APIServers []APIServerConfig

type ExtensionServerConfig struct {
	ClientCAData  string               `json:"clientCAData,omitempty"`
	RequestHeader *RequestHeaderConfig `json:"requestHeaderConfig,omitempty"`
}

type RequestHeaderConfig struct {
	// UsernameHeaders are the headers to check (in order, case-insensitively) for an identity. The first header with a value wins.
	UsernameHeaders []string `json:"usernameHeaders,omitempty"`
	// GroupHeaders are the headers to check (case-insensitively) for a group names.  All values will be used.
	GroupHeaders []string `json:"groupHeaders,omitempty"`
	// ExtraHeaderPrefixes are the head prefixes to check (case-insentively) for filling in
	// the user.Info.Extra.  All values of all matching headers will be added.
	ExtraHeaderPrefixes []string `json:"extraHeaderPrefixes,omitempty"`
	// CAData points to CA bundle file which is used verify the identity of the front proxy
	CAData string `json:"caData"`
	// AllowedClientNames is a list of common names that may be presented by the authenticating front proxy.  Empty means: accept any.
	AllowedClientNames []string `json:"allowedClientNames,omitempty"`
}

func (c ClusterInfo) String() string {
	data, err := yaml.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func (c ClusterInfo) Validate() error {
	var errs []error

	{
		if c.ClientConfig.Insecure {
			errs = append(errs, errors.New("Admission webhooks can't be used when kube apiserver is accesible without verifying its TLS certificate (insecure-skip-tls-verify : true)."))
		} else {
			if c.ExtensionServerConfig.ClientCAData == "" {
				errs = append(errs, errors.Errorf(`"%s/%s" configmap is missing "client-ca-file" key.`, authenticationConfigMapNamespace, authenticationConfigMapName))
			} else if c.ClientConfig.CAData != c.ExtensionServerConfig.ClientCAData {
				errs = append(errs, errors.Errorf(`"%s/%s" configmap has mismatched "client-ca-file" key.`, authenticationConfigMapNamespace, authenticationConfigMapName))
			}

			for _, pod := range c.APIServers {
				if pod.ClientCAData != c.ClientConfig.CAData {
					errs = append(errs, errors.Errorf(`pod "%s" has mismatched "client-ca-file".`, pod.PodName))
				}
			}
		}
	}
	{
		if len(c.APIServers) == 0 && !strings.Contains(c.Version.GitVersion, "-gke.") {
			errs = append(errs, errors.New(`failed to detect kube apiservers. Please file a bug at: https://github.com/appscode/kutil/issues/new .`))
		}
	}
	{
		for _, pod := range c.APIServers {
			certs, err := cert.ParseCertsPEM([]byte(pod.TLSCertData))
			if err != nil {
				errs = append(errs, errors.Wrapf(err, `pod "%s" has bad "tls-cert-file".`, pod.PodName))
			} else {
				cert := certs[0]

				var intermediates *x509.CertPool
				if len(certs) > 1 {
					intermediates = x509.NewCertPool()
					for _, ic := range certs[1:] {
						intermediates.AddCert(ic)
					}
				}

				roots := x509.NewCertPool()
				ok := roots.AppendCertsFromPEM([]byte(pod.ClientCAData))
				if !ok {
					errs = append(errs, errors.Errorf(`pod "%s" has bad "client-ca-file".`, pod.PodName))
					continue
				}

				opts := x509.VerifyOptions{
					Roots:         roots,
					Intermediates: intermediates,
				}
				if _, err := cert.Verify(opts); err != nil {
					errs = append(errs, errors.Wrapf(err, `failed to verify tls-cert-file of pod "%s".`, pod.PodName))
				}
			}
		}
	}
	{
		if c.ExtensionServerConfig.RequestHeader == nil {
			errs = append(errs, errors.Errorf(`"%s/%s" configmap is missing "requestheader-client-ca-file" key.`, authenticationConfigMapNamespace, authenticationConfigMapName))
		}
		for _, pod := range c.APIServers {
			if pod.RequestHeaderCAData != c.ExtensionServerConfig.RequestHeader.CAData {
				errs = append(errs, errors.Errorf(`pod "%s" has mismatched "requestheader-client-ca-file".`, pod.PodName))
			}
		}
	}
	{
		for _, pod := range c.APIServers {
			modes := sets.NewString(pod.AuthorizationMode...)
			if !modes.Has("RBAC") {
				errs = append(errs, errors.Errorf(`pod "%s" does not enable RBAC authorization mode.`, pod.PodName))
			}
		}
	}
	{
		for _, pod := range c.APIServers {
			adms := sets.NewString(pod.AdmissionControl...)
			if !adms.Has("MutatingAdmissionWebhook") {
				errs = append(errs, errors.Errorf(`pod "%s" does not enable MutatingAdmissionWebhook admission controller.`, pod.PodName))
			}
			if !adms.Has("ValidatingAdmissionWebhook") {
				errs = append(errs, errors.Errorf(`pod "%s" does not enable ValidatingAdmissionWebhook admission controller.`, pod.PodName))
			}
		}
	}
	{
		for _, pod := range c.APIServers {
			if !pod.KubeProxyFound {
				errs = append(errs, errors.Errorf(`pod "%s" is not running kube-proxy.`, pod.PodName))
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (servers APIServers) AdmissionControl(name string) (string, error) {
	if len(servers) == 0 {
		return ErrUnknown.Error(), nil
	}

	n := 0
	for _, s := range servers {
		adms := sets.NewString(s.AdmissionControl...)
		if adms.Has(name) {
			n++
		}
	}

	switch {
	case n == 0:
		return "false", nil
	case n == len(servers):
		return "true", nil
	default:
		return "", errors.Errorf("admission control %s is enabled in %d api server, expected %d", name, n, len(servers))
	}
}

func (servers APIServers) RuntimeConfig(name string) (string, error) {
	nE := 0
	nU := 0
	for _, s := range servers {
		enabled, err := s.RuntimeConfig.Status(name)
		if err == ErrUnknown {
			nU++
		} else if enabled {
			nE++
		}
	}

	switch {
	case nU == len(servers):
		return ErrUnknown.Error(), nil
	case nE == len(servers):
		return "true", nil
	case nE == 0:
		return "false", nil
	default:
		return "", errors.Errorf("%s api is not enabled in all api servers (enabled: %d, unknown: %d, total: %d)", name, nE, nU, len(servers))
	}
}

func (servers APIServers) FeatureGate(name string) (string, error) {
	nE := 0
	nU := 0
	for _, s := range servers {
		enabled, err := s.FeatureGates.Status(name)
		if err == ErrUnknown {
			nU++
		} else if enabled {
			nE++
		}
	}

	switch {
	case nU == len(servers):
		return ErrUnknown.Error(), nil
	case nE == len(servers):
		return "true", nil
	case nE == 0:
		return "false", nil
	default:
		return "", errors.Errorf("%s api is not enabled in all api servers (enabled: %d, unknown: %d, total: %d)", name, nE, nU, len(servers))
	}
}
