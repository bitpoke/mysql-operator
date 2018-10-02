package repos

type DebianRepo struct {
	Listing   string
	Deb       string
	DebSrc    string
	GPGKeyURL string
}

// https://repo.saltstack.com/#debian
var DebianRepos map[string]DebianRepo = map[string]DebianRepo{
	// Debian Jessie
	"saltstack-2016.11.1-debian-8": {
		Listing:   "/etc/apt/sources.list.d/saltstack.list",
		Deb:       "deb http://repo.saltstack.com/apt/debian/8/amd64/archive/2016.11.1 jessie main",
		GPGKeyURL: "https://repo.saltstack.com/apt/debian/8/amd64/archive/2016.11.1/SALTSTACK-GPG-KEY.pub",
	},
	"saltstack-2015.8.8-debian-8": {
		Listing:   "/etc/apt/sources.list.d/saltstack.list",
		Deb:       "deb http://repo.saltstack.com/apt/debian/8/amd64/archive/2015.8.8 jessie main",
		GPGKeyURL: "https://repo.saltstack.com/apt/debian/8/amd64/archive/2015.8.8/SALTSTACK-GPG-KEY.pub",
	},
	// Ubuntu
	"saltstack-2016.11.1-ubuntu-16.04": {
		Listing:   "/etc/apt/sources.list.d/saltstack.list",
		Deb:       "deb http://repo.saltstack.com/apt/ubuntu/16.04/amd64/archive/2016.11.1 xenial main",
		GPGKeyURL: "https://repo.saltstack.com/apt/ubuntu/16.04/amd64/archive/2016.11.1/SALTSTACK-GPG-KEY.pub",
	},
	"saltstack-2015.8-ubuntu-14.04": {
		Listing:   "/etc/apt/sources.list.d/saltstack.list",
		Deb:       "deb http://repo.saltstack.com/apt/ubuntu/14.04/amd64/2015.8 trusty main",
		GPGKeyURL: "https://repo.saltstack.com/apt/ubuntu/14.04/amd64/latest/SALTSTACK-GPG-KEY.pub",
	},
	"gcsfuse": {
		Listing:   "/etc/apt/sources.list.d/gcsfuse.list",
		Deb:       "deb http://packages.cloud.google.com/apt gcsfuse-jessie main",
		GPGKeyURL: "https://packages.cloud.google.com/apt/doc/apt-key.gpg",
	},
}

var (
	DefaultSaltstackVersion = map[string]string{
		"debian": "saltstack-2016.11.1-debian-8",
		"ubuntu": "saltstack-2016.11.1-ubuntu-16.04",
	}
)
