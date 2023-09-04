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

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = utils.SIGDescribe("StorageClasses", func() {

	f := framework.NewDefaultFramework("csi-storageclass")
	f.NamespacePodSecurityLevel = admissionapi.LevelBaseline

	ginkgo.Describe("CSI Conformance", func() {
		ginkgo.It("should run through the lifecycle of a StorageClass", func(ctx context.Context) {

			scClient := f.ClientSet.StorageV1().StorageClasses()
			var initialSC, replacementSC *storagev1.StorageClass

			initialSC = &storagev1.StorageClass{
				TypeMeta: metav1.TypeMeta{
					Kind: "StorageClass",
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "e2e-",
				},
				Provisioner: "e2e-fake-provisioner",
			}

			ginkgo.By("Creating a StorageClass")
			createdStorageClass, err := scClient.Create(ctx, initialSC, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			ginkgo.By(fmt.Sprintf("Get StorageClass %q", createdStorageClass.Name))
			retrievedStorageClass, err := scClient.Get(ctx, createdStorageClass.Name, metav1.GetOptions{})
			framework.ExpectNoError(err)

			ginkgo.By(fmt.Sprintf("Patching the StorageClass %q", retrievedStorageClass.Name))
			payload := "{\"metadata\":{\"labels\":{\"" + retrievedStorageClass.Name + "\":\"patched\"}}}"
			patchedStorageClass, err := scClient.Patch(ctx, retrievedStorageClass.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
			framework.ExpectNoError(err, "Failed to patch StorageClass %q", retrievedStorageClass.Name)
			gomega.Expect(patchedStorageClass.Labels).To(gomega.HaveKeyWithValue(patchedStorageClass.Name, "patched"), "Checking that patched label has been applied")

			ginkgo.By(fmt.Sprintf("Delete StorageClass %q", patchedStorageClass.Name))
			err = scClient.Delete(ctx, patchedStorageClass.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			ginkgo.By("Create a replacement StorageClass")

			replacementSC = &storagev1.StorageClass{
				TypeMeta: metav1.TypeMeta{
					Kind: "StorageClass",
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "e2e-v2-",
				},
				Provisioner: "e2e-fake-provisioner",
			}

			replacementStorageClass, err := scClient.Create(ctx, replacementSC, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			ginkgo.By(fmt.Sprintf("Updating StorageClass %q", replacementStorageClass.Name))
			var updatedStorageClass *storagev1.StorageClass

			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				sc, err := scClient.Get(ctx, replacementStorageClass.Name, metav1.GetOptions{})
				framework.ExpectNoError(err, "Unable to get Storage %q", replacementStorageClass.Name)
				sc.Labels = map[string]string{replacementStorageClass.Name: "updated"}
				updatedStorageClass, err = scClient.Update(ctx, sc, metav1.UpdateOptions{})

				return err
			})
			framework.ExpectNoError(err, "failed to update StorageClass %q", replacementStorageClass.Name)
			gomega.Expect(updatedStorageClass.Labels).To(gomega.HaveKeyWithValue(replacementStorageClass.Name, "updated"), "Checking that updated label has been applied")

			scSelector := labels.Set{replacementStorageClass.Name: "updated"}.AsSelector().String()
			ginkgo.By(fmt.Sprintf("Listing all StorageClass with the labelSelector: %q", scSelector))
			scList, err := scClient.List(ctx, metav1.ListOptions{LabelSelector: scSelector})
			framework.ExpectNoError(err, "Failed to list StorageClasses with the labelSelector: %q", scSelector)
			gomega.Expect(scList.Items).To(gomega.HaveLen(1))

			ginkgo.By(fmt.Sprintf("Deleting StorageClass %q via DeleteCollection", updatedStorageClass.Name))
			err = scClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: scSelector})
			framework.ExpectNoError(err, "Failed to delete StorageClass %q", updatedStorageClass.Name)
		})
	})
})
