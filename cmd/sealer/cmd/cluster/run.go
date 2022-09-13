// Copyright © 2021 Alibaba Group Holding Ltd.
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

package cluster

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/sealerio/sealer/apply"
	"github.com/sealerio/sealer/cmd/sealer/cmd/utils"
	"github.com/sealerio/sealer/common"
	clusterruntime "github.com/sealerio/sealer/pkg/cluster-runtime"
	"github.com/sealerio/sealer/pkg/clusterfile"
	imagecommon "github.com/sealerio/sealer/pkg/define/options"
	"github.com/sealerio/sealer/pkg/imagedistributor"
	"github.com/sealerio/sealer/pkg/imageengine"
	"github.com/sealerio/sealer/pkg/infradriver"
	"github.com/sealerio/sealer/utils/strings"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var runArgs *apply.Args

var exampleForRunCmd = `
create cluster to your bare metal server, appoint the iplist:
	sealer run kubernetes:v1.19.8 --masters 192.168.0.2,192.168.0.3,192.168.0.4 \
		--nodes 192.168.0.5,192.168.0.6,192.168.0.7 --passwd xxx
specify server SSH port :
  All servers use the same SSH port (default port: 22):
	sealer run kubernetes:v1.19.8 --masters 192.168.0.2,192.168.0.3,192.168.0.4 \
	--nodes 192.168.0.5,192.168.0.6,192.168.0.7 --port 24 --passwd xxx
  Different SSH port numbers exist:
	sealer run kubernetes:v1.19.8 --masters 192.168.0.2,192.168.0.3:23,192.168.0.4:24 \
	--nodes 192.168.0.5:25,192.168.0.6:25,192.168.0.7:27 --passwd xxx
create a cluster with custom environment variables:
	sealer run -e DashBoardPort=8443 mydashboard:latest  --masters 192.168.0.2,192.168.0.3,192.168.0.4 \
	--nodes 192.168.0.5,192.168.0.6,192.168.0.7 --passwd xxx
`

func NewRunCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "run",
		Short:   "start to run a cluster from a ClusterImage",
		Long:    `sealer run registry.cn-qingdao.aliyuncs.com/sealer-io/kubernetes:v1.19.8 --masters [arg] --nodes [arg]`,
		Example: exampleForRunCmd,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: remove this now, maybe we can support it later
			// set local ip address as master0 default ip if user input is empty.
			// this is convenient to execute `sealer run` without set many arguments.
			// Example looks like "sealer run kubernetes:v1.19.8"
			//if runArgs.Masters == "" {
			//	ip, err := net.GetLocalDefaultIP()
			//	if err != nil {
			//		return err
			//	}
			//	runArgs.Masters = ip
			//}

			//todo merge args from commandline , write to disk.
			var (
				cf              clusterfile.Interface
				clusterFileData []byte
				err             error
			)
			if clusterFile != "" {
				clusterFileData, err = ioutil.ReadFile(filepath.Clean(clusterFile))
				if err != nil {
					return err
				}
				cf, err = clusterfile.NewClusterFile(clusterFileData)
				if err != nil {
					return err
				}
			} else {
				if err := utils.ValidateRunArgs(runArgs); err != nil {
					return fmt.Errorf("failed to validate input run args: %v", err)
				}
				resultHosts, err := utils.GetHosts(runArgs.Masters, runArgs.Nodes)
				if err != nil {
					return err
				}
				cluster, err := utils.ConstructClusterFromArg(args[0], runArgs, resultHosts)
				if err != nil {
					return err
				}
				clusterData, err := yaml.Marshal(cluster)
				if err != nil {
					return err
				}
				cf, err = clusterfile.NewClusterFile(clusterData)
				if err != nil {
					return err
				}
			}

			cluster := cf.GetCluster()

			infraDriver, err := infradriver.NewInfraDriver(&cluster)
			if err != nil {
				return err
			}

			imageEngine, err := imageengine.NewImageEngine(imagecommon.EngineGlobalConfigurations{})
			if err != nil {
				return err
			}

			distributor, err := imagedistributor.NewScpDistributor(imageEngine, infraDriver)
			if err != nil {
				return err
			}

			// distribute rootfs
			if err = distributor.Distribute(cluster.Spec.Image, infraDriver.GetHostIPList()); err != nil {
				return err
			}

			runtimeConfig := new(clusterruntime.RuntimeConfig)
			if cf.GetPlugins() != nil {
				runtimeConfig.Plugins = cf.GetPlugins()
			}

			if cf.GetKubeadmConfig() != nil {
				runtimeConfig.KubeadmConfig = *cf.GetKubeadmConfig()
			}

			installer, err := clusterruntime.NewInstaller(infraDriver, imageEngine, *runtimeConfig)

			if err != nil {
				return err
			}

			_, _, err = installer.Install()
			if err != nil {
				return err
			}

			if err = cf.SaveAll(); err != nil {
				return err
			}

			// TODO install APP
			// todo render app env data
			// todo dump app config
			return nil

		},
	}
	runArgs = &apply.Args{}
	runCmd.Flags().StringVarP(&runArgs.Provider, "provider", "", "", "set infra provider, example `ALI_CLOUD`, the local server need ignore this")
	runCmd.Flags().StringVarP(&runArgs.Masters, "masters", "m", "", "set count or IPList to masters")
	runCmd.Flags().StringVarP(&runArgs.Nodes, "nodes", "n", "", "set count or IPList to nodes")
	runCmd.Flags().StringVar(&runArgs.ClusterName, "cluster-name", "my-cluster", "set cluster name")
	runCmd.Flags().StringVarP(&runArgs.User, "user", "u", "root", "set baremetal server username")
	runCmd.Flags().StringVarP(&runArgs.Password, "passwd", "p", "", "set cloud provider or baremetal server password")
	runCmd.Flags().Uint16Var(&runArgs.Port, "port", 22, "set the sshd service port number for the server (default port: 22)")
	runCmd.Flags().StringVar(&runArgs.Pk, "pk", filepath.Join(common.GetHomeDir(), ".ssh", "id_rsa"), "set baremetal server private key")
	runCmd.Flags().StringVar(&runArgs.PkPassword, "pk-passwd", "", "set baremetal server private key password")
	runCmd.Flags().StringSliceVar(&runArgs.CMDArgs, "cmd-args", []string{}, "set args for image cmd instruction")
	runCmd.Flags().StringSliceVarP(&runArgs.CustomEnv, "env", "e", []string{}, "set custom environment variables")
	err := runCmd.RegisterFlagCompletionFunc("provider", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return strings.ContainPartial([]string{common.BAREMETAL, common.AliCloud, common.CONTAINER}, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	if err != nil {
		logrus.Errorf("provide completion for provider flag, err: %v", err)
		os.Exit(1)
	}
	return runCmd
}