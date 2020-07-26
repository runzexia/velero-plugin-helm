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
	"k8s.io/client-go/kubernetes"
	storage "k8s.io/helm/pkg/storage/driver"
)

type storageFactory interface {
	Storage(namespace string) storage.Driver
	Name() string
}

type secretsStorage struct {
	kubeClient kubernetes.Interface
}

func (s *secretsStorage) Storage(namespace string) storage.Driver {
	secrets := s.kubeClient.CoreV1().
		Secrets(namespace)
	return storage.NewSecrets(secrets)
}

func (c *secretsStorage) Name() string { return "secrets" }

type configmapsStorage struct {
	kubeClient kubernetes.Interface
}

func (c *configmapsStorage) Storage(namespace string) storage.Driver {
	configmaps := c.kubeClient.CoreV1().
		ConfigMaps(namespace)
	return storage.NewConfigMaps(configmaps)
}

func (c *configmapsStorage) Name() string { return "configmaps" }
