/*
Copyright 2017, 2019 the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import (
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/client"
	vdiscvoery "github.com/vmware-tanzu/velero/pkg/discovery"
	"gopkg.in/yaml.v2"
	"io"
	"k8s.io/apimachinery/pkg/runtime/schema"
	storage "k8s.io/helm/pkg/storage/driver"
	"strconv"
	"strings"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

// BackupPlugin is a backup item action for helm chart.
type BackupPlugin struct {
	clientset clientset.Interface
	log       logrus.FieldLogger
	storage   storageFactory
}

func NewBackupPlugin(f client.Factory, resource string) func(logrus.FieldLogger) (interface{}, error) {
	return func(logger logrus.FieldLogger) (interface{}, error) {
		kubeClient, err := f.KubeClient()
		if err != nil {
			return nil, err
		}
		clientset, err := f.Client()
		if err != nil {
			return nil, err
		}
		var sf storageFactory
		switch resource {
		case "configmaps":
			sf = &configmapsStorage{kubeClient}
		case "secrets":
			sf = &secretsStorage{kubeClient}
		}
		return &BackupPlugin{clientset: clientset, log: logger, storage: sf}, nil
	}
}

// AppliesTo returns configmaps/secrets that are deployed and owned by tiller.
func (p *BackupPlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{p.storage.Name()},
		LabelSelector:     "OWNER=TILLER",
	}, nil
}

type manifest struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

func (r *releaseBackup) resourceNamespace(apiResource *metav1.APIResource) string {
	if apiResource.Namespaced {
		return r.release.GetNamespace()
	}
	return ""
}

func (r *releaseBackup) fromManifest(manifestString string) ([]velero.ResourceIdentifier, error) {
	var resources []velero.ResourceIdentifier
	dec := yaml.NewDecoder(strings.NewReader(manifestString))
	for {
		var m manifest
		err := dec.Decode(&m)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		gv, err := schema.ParseGroupVersion(m.ApiVersion)
		if err != nil {
			return nil, err
		}
		gvr, apiResource, err := r.dHelper.KindFor(gv.WithKind(m.Kind))
		if err != nil {
			return nil, err
		}

		resources = append(resources, velero.ResourceIdentifier{
			GroupResource: schema.GroupResource{
				Group:    gvr.Group,
				Resource: gvr.Resource,
			},
			Namespace: r.resourceNamespace(&apiResource),
			Name:      m.Metadata.Name,
		})
	}
	return resources, nil
}


func (r *releaseBackup) hookResources(hook *rspb.Hook) ([]velero.ResourceIdentifier, error) {
	// Hook never ran, skip it
	if hook.GetLastRun().GetSeconds() == 0 {
		return nil, nil
	}
	for _, p := range hook.GetDeletePolicies() {
		// TODO: If hook has any other delete policies
		// aside from before-hook-creation we need to check
		// with kubernetes if it actually still exists, for now
		// hooks with delete policy other than before-hook-creation
		// will be skipped
		if p != rspb.Hook_BEFORE_HOOK_CREATION {
			return nil, nil
		}
	}
	return r.fromManifest(hook.GetManifest())
}

type releaseBackup struct {
	metadata     metav1.Object
	log          logrus.FieldLogger
	driver       storage.Driver
	release      *rspb.Release
	dHelper      vdiscvoery.Helper
}

func (r *releaseBackup) runReleaseBackup() ([]velero.ResourceIdentifier, error) {
	relVer, err := r.driver.Get(r.metadata.GetName())
	if err != nil {
		return nil, err
	}
	r.release = relVer

	resources := make([]velero.ResourceIdentifier, 0)

	// Only backup resources for releases that are deployed
	if relVer.GetInfo().GetStatus().GetCode() == rspb.Status_DEPLOYED {
		for _, hook := range relVer.GetHooks() {
			hookResources, err := r.hookResources(hook)
			if err != nil {
				return nil, err
			}
			resources = append(resources, hookResources...)
		}
		releaseResources, err := r.fromManifest(relVer.GetManifest())
		if err != nil {
			return nil, err
		}
		resources = append(resources, releaseResources...)
	}
	resources = append(resources, velero.ResourceIdentifier{
		GroupResource: schema.GroupResource{
			Resource: r.driver.Name(),
		},
		Namespace: r.metadata.GetNamespace(),
		Name:      relVer.GetName() + "." + "v" + strconv.FormatInt(int64(relVer.GetVersion()), 10),
	})

	return resources, nil
}


func (p *BackupPlugin) getIdentifiers(metadata metav1.Object) ([]velero.ResourceIdentifier, error) {
	driver := p.storage.Storage(metadata.GetNamespace())
	discoveryClient := p.clientset.Discovery()

	helper, err := vdiscvoery.NewHelper(discoveryClient,p.log)
	if err != nil {
		return nil, err
	}
	releaseBackup := releaseBackup{
		metadata:     metadata,
		driver:       driver,
		log:          p.log,
		dHelper: helper,
	}
	return releaseBackup.runReleaseBackup()
}

// Execute returns chart configmap/secret allong with all additional resources defined by chart.
func (p *BackupPlugin) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	metadata, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, err
	}
	identifiers, err := p.getIdentifiers(metadata)
	return item, identifiers, err
}
