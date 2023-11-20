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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	"github.com/onsi/ginkgo/v2"
	// "github.com/onsi/gomega"
)

var _ = utils.SIGDescribe("CSINodes", func() {

	f := framework.NewDefaultFramework("csinodes")

	ginkgo.Describe("CSI Conformance", func() {

		ginkgo.It("tkt47", func(ctx context.Context) {

			csiNodeList, err := f.ClientSet.StorageV1().CSINodes().List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err)

			framework.Logf("csiNodeList: %#v", csiNodeList)
			firstCSINode := csiNodeList.Items[0]

			csiNode, err := f.ClientSet.StorageV1().CSINodes().Get(ctx, firstCSINode.Name, metav1.GetOptions{})
			framework.ExpectNoError(err)
			framework.Logf("csiNode: %#v", csiNode)
		})
	})
})
