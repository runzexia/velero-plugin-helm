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

package main

import (
	"github.com/runzexia/velero-plugin-helm/pkg/plugin"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	f := client.NewFactory("velero-plugin-helm", client.VeleroConfig{})
	srv := framework.NewServer()
	for _, resource := range []string{"configmaps", "secrets"} {
		srv.RegisterBackupItemAction("velero-plugin-helm/"+resource, plugin.NewBackupPlugin(f, resource))
	}
	srv.Serve()
}
