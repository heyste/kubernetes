/*
Copyright 2023 The Kubernetes Authors.

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

package storage

import (
	"context"
	"fmt"
	"math/rand"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	"github.com/onsi/ginkgo/v2"
	// "github.com/onsi/gomega"
)

var _ = utils.SIGDescribe("VolumeAttachment", func() {

	f := framework.NewDefaultFramework("volumeattachment")

	ginkgo.Describe("Conformance", func() {

		ginkgo.It("tkt53", func(ctx context.Context) {

			randUID := "e2e-" + utilrand.String(5)
			vaName := "va-" + randUID
			pvName := "pv-" + randUID

			nodes, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err)
			randNode := rand.Intn(len(nodes.Items))
			vaNodeName := nodes.Items[randNode].Name
			vaAttachStatus := false

			ginkgo.By(fmt.Sprintf("Create VolumeAttachment %q on node %q", vaName, vaNodeName))
			newVa := NewVolumeAttachment(vaName, pvName, vaNodeName, vaAttachStatus)

			createdVA, err := f.ClientSet.StorageV1().VolumeAttachments().Create(ctx, newVa, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			framework.Logf("CreatedVA: %#v", createdVA)

			ginkgo.By(fmt.Sprintf("Get VolumeAttachment %q on node %q", vaName, vaNodeName))
			retrievedVA, err := f.ClientSet.StorageV1().VolumeAttachments().Get(ctx, vaName, metav1.GetOptions{})
			framework.ExpectNoError(err)
			framework.Logf("RetrievedVA: %#v", retrievedVA)

			ginkgo.By("List VolumeAttachments")
			listVolumeAttachments, err := f.ClientSet.StorageV1().VolumeAttachments().List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err)

			framework.Logf("list VolumeAttachments: %#v", listVolumeAttachments)

			ginkgo.By(fmt.Sprintf("Delete VolumeAttachment %q on node %q", vaName, vaNodeName))
			err = f.ClientSet.StorageV1().VolumeAttachments().Delete(ctx, vaName, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

		})
	})
})

func NewVolumeAttachment(vaName, pvName, nodeName string, status bool) *storagev1.VolumeAttachment {
	return &storagev1.VolumeAttachment{

		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID(vaName),
			Name: vaName,
		},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: "e2e-test.storage.k8s.io",
			NodeName: nodeName,
			Source: storagev1.VolumeAttachmentSource{
				PersistentVolumeName: &pvName,
			},
		},
		Status: storagev1.VolumeAttachmentStatus{
			Attached: status,
		},
	}
}
