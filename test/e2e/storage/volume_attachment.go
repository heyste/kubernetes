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
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = utils.SIGDescribe("VolumeAttachment", func() {

	f := framework.NewDefaultFramework("volumeattachment")

	ginkgo.Describe("Conformance", func() {

		ginkgo.It("should run through the lifecycle of a VolumeAttachment", func(ctx context.Context) {

			vaClient := f.ClientSet.StorageV1().VolumeAttachments()

			randUID := "e2e-" + utilrand.String(5)
			vaName := "va-" + randUID
			pvName := "pv-" + randUID

			nodes, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err, "failed to list nodes")
			randNode := rand.Intn(len(nodes.Items))
			vaNodeName := nodes.Items[randNode].Name
			vaAttachStatus := false

			ginkgo.By(fmt.Sprintf("Create VolumeAttachment %q on node %q", vaName, vaNodeName))
			initialVA := NewVolumeAttachment(vaName, pvName, vaNodeName, vaAttachStatus)

			createdVA, err := vaClient.Create(ctx, initialVA, metav1.CreateOptions{})
			framework.ExpectNoError(err, "failed to create VolumeAttachment %q", vaName)
			gomega.Expect(createdVA.Name).To(gomega.Equal(vaName), "Checking that the created VolumeAttachment has the correct name")

			ginkgo.By(fmt.Sprintf("Get VolumeAttachment %q on node %q", vaName, vaNodeName))
			retrievedVA, err := vaClient.Get(ctx, vaName, metav1.GetOptions{})
			framework.ExpectNoError(err, "failed to get VolumeAttachment %q", vaName)
			gomega.Expect(retrievedVA.Name).To(gomega.Equal(vaName), "Checking that create VolumeAttachment has the correct name")

			ginkgo.By(fmt.Sprintf("Patch VolumeAttachment %q on node %q", vaName, vaNodeName))
			payload := "{\"metadata\":{\"labels\":{\"" + retrievedVA.Name + "\":\"patched\"}}}"
			patchedVA, err := vaClient.Patch(ctx, retrievedVA.Name, types.MergePatchType, []byte(payload), metav1.PatchOptions{})
			framework.ExpectNoError(err, "failed to patch PV %q", vaName)
			gomega.Expect(patchedVA.Labels).To(gomega.HaveKeyWithValue(patchedVA.Name, "patched"), "Checking that patched label has been applied")

			ginkgo.By(fmt.Sprintf("List VolumeAttachments with %q label", patchedVA.Name+"=patched"))
			vaList, err := vaClient.List(ctx, metav1.ListOptions{LabelSelector: patchedVA.Name + "=patched"})
			framework.ExpectNoError(err, "failed to list VolumeAttachments")
			gomega.Expect(vaList.Items).To(gomega.HaveLen(1))

			ginkgo.By(fmt.Sprintf("Delete VolumeAttachment %q on node %q", vaName, vaNodeName))
			err = vaClient.Delete(ctx, vaName, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete VolumeAttachment %q", vaName)

			randUID = "e2e-" + utilrand.String(5)
			vaName = "va-" + randUID
			pvName = "pv-" + randUID

			ginkgo.By(fmt.Sprintf("Create replacement VolumeAttachment %q on node %q", vaName, vaNodeName))
			secondVA := NewVolumeAttachment(vaName, pvName, vaNodeName, vaAttachStatus)

			replacementVA, err := vaClient.Create(ctx, secondVA, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			gomega.Expect(replacementVA.Name).To(gomega.Equal(vaName), "Checking that the replacement VolumeAttachment has the correct name")

			ginkgo.By(fmt.Sprintf("Update the VolumeAttachment %q on node %q", replacementVA.Name, vaNodeName))
			var updatedVA *storagev1.VolumeAttachment

			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				currentVA, err := vaClient.Get(ctx, replacementVA.Name, metav1.GetOptions{})
				framework.ExpectNoError(err, "failed to get VolumeAttachment %q", patchedVA.Name)
				currentVA.Labels = map[string]string{replacementVA.Name: "updated"}
				updatedVA, err = vaClient.Update(ctx, currentVA, metav1.UpdateOptions{})

				return err
			})
			framework.ExpectNoError(err, "failed to update VolumeAttachment %q on node %q", replacementVA.Name, vaNodeName)
			gomega.Expect(updatedVA.Labels).To(gomega.HaveKeyWithValue(updatedVA.Name, "updated"), "Checking that updated label has been applied")

			ginkgo.By(fmt.Sprintf("DeleteCollection of VolumeAttachments with %q label", replacementVA.Name+"=updated"))
			err = vaClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: replacementVA.Name + "=updated"})
			framework.ExpectNoError(err, "failed to delete VolumeAttachment collection")
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
