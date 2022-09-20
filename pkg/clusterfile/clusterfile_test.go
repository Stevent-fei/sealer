// Copyright © 2022 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clusterfile

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/sealerio/sealer/common"
)

func TestSaveAll(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test set cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterfile := "test/testclusterfile.yaml"
			clusterFileData, err := ioutil.ReadFile(filepath.Clean(clusterfile))
			if err != nil {
				t.Errorf("failed to read file error:(%v)", err)
			}
			cf, err := NewClusterFile(clusterFileData)
			if err != nil {
				t.Errorf("failed to get cluster file data error:(%v)", err)
			}
			cluster := cf.GetCluster()
			env := "a=b,b=c,c=d"
			cluster.Spec.Env = append(cluster.Spec.Env, env)
			configs := cf.GetConfigs()
			fmt.Println(configs)
			plugins := cf.GetPlugins()
			fmt.Println(plugins)
			config := cf.GetKubeadmConfig()
			config.InitConfiguration.TypeMeta.Kind = common.InitConfiguration
			cf.SetCluster(cluster)
			if err := cf.SaveAll(); err != nil {
				t.Errorf("failed to save all error:(%v)", err)
			}
		})
	}
}
