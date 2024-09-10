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
	"time"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = utils.SIGDescribe("CSINodes", func() {

	f := framework.NewDefaultFramework("csinodes")

	ginkgo.Describe("CSI Conformance", func() {

		ginkgo.It("tkt47", func(ctx context.Context) {

			nodeClient := f.ClientSet.CoreV1().Nodes()
			csiNodeClient := f.ClientSet.StorageV1().CSINodes()

			initialFakeNode := v1.Node{

				ObjectMeta: metav1.ObjectMeta{
					Name: "e2e-fake-node-" + utilrand.String(5),
				},
				Status: v1.NodeStatus{
					Phase: v1.NodeRunning,
					Conditions: []v1.NodeCondition{
						{
							Status:  v1.ConditionTrue,
							Message: "Set from e2e test",
							Reason:  "E2E",
							Type:    v1.NodeReady,
						},
					},
				},
			}

			ginkgo.By(fmt.Sprintf("Creating initial node %q", initialFakeNode.Name))
			createdNode, err := nodeClient.Create(ctx, &initialFakeNode, metav1.CreateOptions{})
			framework.ExpectNoError(err, "failed to create node %q", initialFakeNode.Name)
			gomega.Expect(createdNode.Name).To(gomega.Equal(initialFakeNode.Name), "Checking that the node has been created")

			ginkgo.By(fmt.Sprintf("Getting initial node: %q", initialFakeNode.Name))
			retrievedNode, err := nodeClient.Get(ctx, initialFakeNode.Name, metav1.GetOptions{})
			framework.ExpectNoError(err, "Failed to retrieve node %q", initialFakeNode.Name)
			gomega.Expect(retrievedNode.Name).To(gomega.Equal(initialFakeNode.Name), "Checking that the retrieved name has been found")

			initialCSINode := storagev1.CSINode{
				ObjectMeta: metav1.ObjectMeta{
					Name: initialFakeNode.Name,
				},
			}

			ginkgo.By(fmt.Sprintf("Creating initial csiNode %q", initialCSINode.Name))
			csiNode, err := csiNodeClient.Create(ctx, &initialCSINode, metav1.CreateOptions{})
			framework.ExpectNoError(err, "failed to create csiNode %q", initialFakeNode.Name)

			ginkgo.By(fmt.Sprintf("Getting initial csiNode %q", initialCSINode.Name))
			retrievedCSINode, err := csiNodeClient.Get(ctx, initialCSINode.Name, metav1.GetOptions{})
			framework.ExpectNoError(err, "Failed to retrieve csiNode %q", initialFakeNode.Name)
			gomega.Expect(retrievedCSINode.Name).To(gomega.Equal(csiNode.Name), "Checking that the retrieved name has been found")

			ginkgo.By(fmt.Sprintf("Patching initial csiNode: %q", initialCSINode.Name))
			payload := "{\"metadata\":{\"labels\":{\"" + csiNode.Name + "\":\"patched\"}}}"
			patchedCSINode, err := csiNodeClient.Patch(ctx, csiNode.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
			framework.ExpectNoError(err, "Failed to patch csiNode %q", csiNode.Name)
			gomega.Expect(patchedCSINode.Labels).To(gomega.HaveKeyWithValue(csiNode.Name, "patched"), "Checking that patched label has been applied")

			patchedSelector := labels.Set{csiNode.Name: "patched"}.AsSelector().String()
			ginkgo.By(fmt.Sprintf("Listing csiNodes with LabelSelector %q", patchedSelector))
			csiNodeList, err := csiNodeClient.List(ctx, metav1.ListOptions{LabelSelector: patchedSelector})
			framework.ExpectNoError(err, "failed to list csiNodes")
			gomega.Expect(csiNodeList.Items).To(gomega.HaveLen(1))

			ginkgo.By(fmt.Sprintf("Delete initial csiNode: %q", initialCSINode.Name))
			err = csiNodeClient.Delete(ctx, csiNode.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete node %q", csiNode.Name)

			ginkgo.By(fmt.Sprintf("Confirm deletion of csiNode with LabelSelector %q", patchedSelector))
			csiNodeList, err = csiNodeClient.List(ctx, metav1.ListOptions{LabelSelector: patchedSelector})
			framework.ExpectNoError(err, "failed to list csiNodes")
			gomega.Expect(csiNodeList.Items).To(gomega.HaveLen(0))

			ginkgo.By(fmt.Sprintf("Delete initial node %q", initialFakeNode.Name))
			err = nodeClient.Delete(ctx, initialFakeNode.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete node")

			ginkgo.By(fmt.Sprintf("Confirm deletion of node %q", initialFakeNode.Name))
			gomega.Eventually(ctx, func(ctx context.Context) error {
				_, err := nodeClient.Get(ctx, initialFakeNode.Name, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return fmt.Errorf("nodeClient.Get returned an unexpected error: %w", err)
				}
				return fmt.Errorf("node still exists: %s", initialFakeNode.Name)
			}, 3*time.Minute, 5*time.Second).Should(gomega.Succeed(), "Timeout while waiting to confirm Node deletion")

			replacementFakeNode := v1.Node{

				ObjectMeta: metav1.ObjectMeta{
					Name: "e2e-fake-node-" + utilrand.String(5),
				},
				Status: v1.NodeStatus{
					Phase: v1.NodeRunning,
					Conditions: []v1.NodeCondition{
						{
							Status:  v1.ConditionTrue,
							Message: "Set from e2e test",
							Reason:  "E2E",
							Type:    v1.NodeReady,
						},
					},
				},
			}

			ginkgo.By(fmt.Sprintf("Creating replacement node %q", replacementFakeNode.Name))
			replacementNode, err := nodeClient.Create(ctx, &replacementFakeNode, metav1.CreateOptions{})
			framework.ExpectNoError(err, "failed to create node %q", replacementFakeNode.Name)
			gomega.Expect(replacementNode.Name).To(gomega.Equal(replacementFakeNode.Name), "Checking that the node has been created")

			ginkgo.By(fmt.Sprintf("Getting replacement node: %q", replacementFakeNode.Name))
			retrievedNode, err = nodeClient.Get(ctx, replacementNode.Name, metav1.GetOptions{})
			framework.ExpectNoError(err, "Failed to retrieve node %q", replacementFakeNode.Name)
			gomega.Expect(retrievedNode.Name).To(gomega.Equal(replacementFakeNode.Name), "Checking that the retrieved name has been found")

			replacementCSINode := storagev1.CSINode{
				ObjectMeta: metav1.ObjectMeta{
					Name: replacementFakeNode.Name,
				},
			}

			ginkgo.By(fmt.Sprintf("Creating replacement csiNode %q", replacementCSINode.Name))
			secondCSINode, err := csiNodeClient.Create(ctx, &replacementCSINode, metav1.CreateOptions{})
			framework.ExpectNoError(err, "failed to create csiNode %q", replacementCSINode.Name)

			ginkgo.By(fmt.Sprintf("Getting replacement csiNode %q", replacementNode.Name))
			retrievedCSINode, err = csiNodeClient.Get(ctx, secondCSINode.Name, metav1.GetOptions{})
			framework.ExpectNoError(err, "Failed to retrieve CSINode %q", replacementFakeNode.Name)
			gomega.Expect(retrievedCSINode.Name).To(gomega.Equal(secondCSINode.Name), "Checking that the retrieved name has been found")

			ginkgo.By(fmt.Sprintf("Updating replacement csiNode %q", retrievedCSINode.Name))
			var updatedCSINode *storagev1.CSINode

			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				tmpCSINode, err := csiNodeClient.Get(ctx, retrievedCSINode.Name, metav1.GetOptions{})
				framework.ExpectNoError(err, "Unable to get %q", replacementCSINode.Name)
				tmpCSINode.Labels = map[string]string{replacementCSINode.Name: "updated"}
				updatedCSINode, err = csiNodeClient.Update(ctx, tmpCSINode, metav1.UpdateOptions{})

				return err
			})
			framework.ExpectNoError(err, "failed to update %q", replacementCSINode.Name)
			gomega.Expect(updatedCSINode.Labels).To(gomega.HaveKeyWithValue(secondCSINode.Name, "updated"), "Checking that updated label has been applied")

			updatedSelector := labels.Set{retrievedCSINode.Name: "updated"}.AsSelector().String()
			err = csiNodeClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: updatedSelector})
			framework.ExpectNoError(err, "failed to delete csiNode Colllection")

			ginkgo.By(fmt.Sprintf("Confirm deletion of replacement csiNode with LabelSelector %q", patchedSelector))
			csiNodeList, err = csiNodeClient.List(ctx, metav1.ListOptions{LabelSelector: patchedSelector})
			framework.ExpectNoError(err, "failed to list csiNodes")
			gomega.Expect(csiNodeList.Items).To(gomega.HaveLen(0))

			ginkgo.By(fmt.Sprintf("Delete replacement node %q", replacementFakeNode.Name))
			err = nodeClient.Delete(ctx, replacementNode.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete node")

			ginkgo.By(fmt.Sprintf("Confirm deletion of replacement node %q", replacementFakeNode.Name))
			gomega.Eventually(ctx, func(ctx context.Context) error {
				_, err := nodeClient.Get(ctx, replacementFakeNode.Name, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return fmt.Errorf("nodeClient.Get returned an unexpected error: %w", err)
				}
				return fmt.Errorf("node still exists: %s", replacementFakeNode.Name)
			}, 3*time.Minute, 5*time.Second).Should(gomega.Succeed(), "Timeout while waiting to confirm Node deletion")
		})
	})
})
