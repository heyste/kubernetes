/*
Copyright 2022 The Kubernetes Authors.

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

package auth

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/admission/api"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/onsi/ginkgo/v2"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = SIGDescribe("SubjectReview", func() {
	f := framework.NewDefaultFramework("subjectreview")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.It("should support SubjectReview API operations", func() {

		AuthClient := f.ClientSet.AuthorizationV1()
		ns := f.Namespace.Name

		ginkgo.By(fmt.Sprintf("Creating SubjectAccessReview in %q namespace", ns))

		verb := "list"
		resourceName := "*"
		resourceGroup := "*"
		resource := "*"
		user := "*"

		sar := &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Group:     resourceGroup,
					Verb:      verb,
					Resource:  resource,
					Namespace: f.Namespace.Name,
					Name:      resourceName,
				},
				User: user,
			},
		}
		framework.Logf("sar: %#v", sar)

		sarResponse, err := AuthClient.SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Unable to create a SubjectAccessReview, %#v", err)
		framework.Logf("sarResponse: %#v", sarResponse)

		ginkgo.By(fmt.Sprintf("Creating a LocalSubjectAccessReview in %q namespace", ns))

		lsar := &authorizationv1.LocalSubjectAccessReview{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
			},
			Spec: authorizationv1.SubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Verb:      "list",
					Group:     api.GroupName,
					Version:   "v1",
					Resource:  "pods",
					Namespace: ns,
				},
				User: "alice",
			},
		}
		framework.Logf("lsar: %#v", lsar)

		framework.Logf("f.Namespace.Name: %#v", ns)
		framework.Logf("lsar.ObjectMeta.Namespace: %#v", lsar.ObjectMeta.Namespace)
		lsarResponse, err := AuthClient.LocalSubjectAccessReviews(ns).Create(context.TODO(), lsar, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Unable to create a LocalSubjectAccessReview, %#v", err)
		framework.Logf("lsarResponse: %#v", lsarResponse)
	})
})
