package mysqlcluster

import (
	kcore "github.com/appscode/kutil/core/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/titanium/pkg/util"
)

func (f *cFactory) syncEnvSecret() (state string, err error) {

	meta := metav1.ObjectMeta{
		Name: f.getNameForResource(EnvSecret),
		Labels: f.getLabels(map[string]string{
			"generated": "true"}),
		OwnerReferences: f.getOwnerReferences(),
	}

	_, act, err := kcore.CreateOrPatchSecret(f.client, meta,
		func(in *core.Secret) *core.Secret {
			in.Data = f.getEnvSecretData(in.Data)
			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) getEnvSecretData(data map[string][]byte) map[string][]byte {
	rUser := []byte(util.RandomString(rStrLen))
	if u, ok := data["TITANIUM_REPLICATION_USER"]; ok && len(u) > 0 {
		rUser = u
	}
	rPass := []byte(util.RandomString(rStrLen))
	if p, ok := data["TITANIUM_REPLICATION_PASSWORD"]; ok && len(p) > 0 {
		rPass = p
	}

	configs := map[string]string{
		"TITANIUM_MASTER_SERVICE":    f.getNameForResource(MasterService),
		"TITANIUM_INIT_BUCKET_URI":   f.cl.Spec.InitBucketURI,
		"TITANIUM_BACKUP_BUCKET_URI": f.cl.Spec.BackupBucketURI,
	}
	fConf := make(map[string][]byte)
	for k, v := range configs {
		fConf[k] = []byte(v)
	}

	fConf["TITANIUM_REPLICATION_USER"] = rUser
	fConf["TITANIUM_REPLICATION_PASSWORD"] = rPass
	return fConf
}
