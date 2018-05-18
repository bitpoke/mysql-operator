package doctor

import (
	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
)

func (d *Doctor) extractVersion(info *ClusterInfo) error {
	v, err := d.kc.Discovery().ServerVersion()
	if err != nil {
		return err
	}

	info.Version = &VersionInfo{
		GitVersion: v.GitVersion,
		GitCommit:  v.GitCommit,
		BuildDate:  v.BuildDate,
		Platform:   v.Platform,
	}

	gv, err := version.NewVersion(v.GitVersion)
	if err != nil {
		return errors.Wrapf(err, "invalid version %s", v.GitVersion)
	}
	mv := gv.ToMutator().ResetMetadata().ResetPrerelease()
	info.Version.Patch = mv.String()
	info.Version.Minor = mv.ResetPatch().String()

	return err
}
