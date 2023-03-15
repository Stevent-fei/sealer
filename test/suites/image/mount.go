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

package image

import (
	"fmt"
	"github.com/containers/buildah"
	"github.com/containers/storage"
	"github.com/olekukonko/tablewriter"
	"github.com/sealerio/sealer/cmd/sealer/cmd/alpha"
	"github.com/sealerio/sealer/pkg/define/options"
	imagebuildah "github.com/sealerio/sealer/pkg/imageengine/buildah"
	"github.com/sealerio/sealer/test/testhelper"
	"github.com/sealerio/sealer/utils/os"
	utilsstrings "github.com/sealerio/sealer/utils/strings"
	"github.com/sirupsen/logrus"
	"strings"
)

func Mount(mountInfo alpha.MountService, name string) error {
	mountPoint, err := mountInfo.Mount(name)
	testhelper.CheckErr(err)

	if ok := os.IsDir(mountPoint); !ok {
		return fmt.Errorf("this directory does not exist")
	}
	return nil
}

func GetContainerID() (string, error) {
	engine, err := imagebuildah.NewBuildahImageEngine(options.EngineGlobalConfigurations{})
	if err != nil {
		testhelper.CheckErr(err)
	}
	store := engine.ImageStore()
	if err != nil {
		testhelper.CheckErr(err)
	}
	clients, err := buildah.OpenAllBuilders(store)
	if err != nil {
		testhelper.CheckErr(err)
	}
	for _, client := range clients {
		mounted, err := client.Mounted()
		if err != nil {
			testhelper.CheckErr(err)
		}
		if mounted {
			return client.ContainerID, nil
		}
	}
	return "", nil
}

func NewMountService() (MountService, error) {
	engine := new(imagebuildah.Engine)
	//engine, err := imagebuildah.NewBuildahImageEngine(options.EngineGlobalConfigurations{})
	store := engine.ImageStore()
	containers, err := store.Containers()
	if err != nil {
		return MountService{}, err
	}
	images, err := store.Images()
	if err != nil {
		return MountService{}, err
	}

	builders, err := buildah.OpenAllBuilders(store)
	if err != nil {
		return MountService{}, err
	}

	return MountService{
		engine:     engine,
		store:      store,
		images:     images,
		containers: containers,
		builders:   builders,
	}, nil
}

type MountService struct {
	table      *tablewriter.Table
	engine     *imagebuildah.Engine
	store      storage.Store
	images     []storage.Image
	containers []storage.Container
	builders   []*buildah.Builder
}

func (m MountService) Mount(imageNameOrID string) (string, error) {
	var imageIDList []string

	for _, builder := range m.builders {
		mounted, err := builder.Mounted()
		if err != nil {
			return "", err
		}
		for _, container := range m.containers {
			if builder.ContainerID == container.ID && mounted {
				imageID := m.getMountedImageID(container)
				imageIDList = append(imageIDList, imageID)
			}
		}
	}

	imageID := m.getImageID(imageNameOrID)
	ok := utilsstrings.IsInSlice(imageID, imageIDList)
	if ok {
		logrus.Warnf("this image has already been mounted, please do not repeat the operation")
		return "", nil
	}
	cid, err := m.engine.CreateContainer(&options.FromOptions{
		Image: imageID,
		Quiet: false,
	})
	if err != nil {
		return "", err
	}
	mounts, err := m.engine.Mount(&options.MountOptions{Containers: []string{cid}})
	if err != nil {
		return "", err
	}
	mountPoint := mounts[0].MountPoint
	logrus.Infof("mount cluster image %s to %s successful", imageNameOrID, mountPoint)

	return mountPoint, nil
}

func (m MountService) getMountedImageID(container storage.Container) string {
	var imageID string
	for _, image := range m.images {
		if container.ImageID == image.ID {
			imageID = image.ID
		}
	}
	return imageID
}

func (m MountService) getImageID(name string) string {
	for _, image := range m.images {
		if strings.HasPrefix(image.ID, name) {
			return image.ID
		}
		for _, n := range image.Names {
			if name == n {
				return image.ID
			}
		}
	}
	return ""
}
