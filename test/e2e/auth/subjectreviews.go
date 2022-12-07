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
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"

	"github.com/onsi/ginkgo/v2"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

var _ = SIGDescribe("SubjectReview", func() {
	f := framework.NewDefaultFramework("subjectreview")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.It("should support SubjectReview API operations", func() {

		AuthClient := f.ClientSet.AuthorizationV1()
		ns := f.Namespace.Name

		podClient := f.ClientSet.CoreV1().Pods(ns)
		podName := "pod-" + utilrand.String(5)
		label := map[string]string{"e2e": podName}

		ginkgo.By(fmt.Sprintf("Create pod %q in namespace %q", podName, ns))
		testPod := e2epod.MustMixinRestrictedPodSecurity(&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   podName,
				Labels: label,
			},
			Spec: v1.PodSpec{
				TerminationGracePeriodSeconds: pointer.Int64(1),
				Containers: []v1.Container{
					{
						Name:  "agnhost",
						Image: imageutils.GetE2EImage(imageutils.Agnhost),
					},
				},
			},
		})
		pod, err := podClient.Create(context.TODO(), testPod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "failed to create Pod %v in namespace %v", testPod.ObjectMeta.Name, ns)
		framework.ExpectNoError(e2epod.WaitForPodRunningInNamespace(f.ClientSet, pod), "Pod didn't start within time out period")

		getPod, err := podClient.Get(context.TODO(), podName, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to get Pod %v in namespace %v", testPod.ObjectMeta.Name, ns)
		framework.Logf("%q in namespace %q is %q", podName, ns, getPod.Status.Phase)

		saName := "system:serviceaccount:" + ns + ":" + pod.Spec.ServiceAccountName
		framework.Logf("serviceaccount name: %q", saName)

		ginkgo.By(fmt.Sprintf("Creating SubjectAccessReview in %q namespace", ns))

		sar := &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Verb:      "get",
					Resource:  "pods",
					Namespace: ns,
					Name:      podName,
					Version:   "v1",
				},
				User: saName,
			},
		}

		sarResponse, err := AuthClient.SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Unable to create a SubjectAccessReview, %#v", err)
		framework.Logf("sarResponse Status: %#v", sarResponse.Status)
		sarAllowed := sarResponse.Status.Allowed

		ginkgo.By(fmt.Sprintf("Creating clientset to impersonate %q", saName))
		config := f.ClientConfig()
		config.Impersonate = rest.ImpersonationConfig{
			UserName: saName,
		}

		impersonatedClientSet, err := kubernetes.NewForConfig(config)
		framework.ExpectNoError(err, "Could not load config, %v", err)

		ginkgo.By(fmt.Sprintf("Verifying api 'get' call to %q as %q", podName, saName))
		_, err = impersonatedClientSet.CoreV1().Pods(ns).Get(context.TODO(), podName, metav1.GetOptions{})

		var verifiedSAR bool
		if err == nil && sarAllowed {
			verifiedSAR = true
			framework.Logf("api call by %q was successful", saName)
		}

		status, ok := err.(*apierrors.StatusError)
		if ok && status.ErrStatus.Code == 403 && !sarAllowed {
			verifiedSAR = true
			framework.Logf("api call by %q was denied", saName)
		}

		if verifiedSAR {
			framework.Logf("SubjectAccessReview has been verified")
		} else {
			framework.Fail(fmt.Sprintf("Could not verify SubjectAccessReview for %q in namespace %q", saName, ns))
		}

		ginkgo.By(fmt.Sprintf("Creating a LocalSubjectAccessReview in %q namespace", ns))

		lsar := &authorizationv1.LocalSubjectAccessReview{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
			},
			Spec: authorizationv1.SubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Verb:      "get",
					Resource:  "pods",
					Namespace: ns,
					Name:      podName,
					Version:   "v1",
				},
				User: saName,
			},
		}

		lsarResponse, err := AuthClient.LocalSubjectAccessReviews(ns).Create(context.TODO(), lsar, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Unable to create a LocalSubjectAccessReview, %#v", err)
		framework.Logf("lsarResponse Status: %#v", lsarResponse.Status)
		lsarAllowed := lsarResponse.Status.Allowed

		ginkgo.By(fmt.Sprintf("Verifying api 'get' call to %q as %q", podName, saName))
		_, err = impersonatedClientSet.CoreV1().Pods(ns).Get(context.TODO(), podName, metav1.GetOptions{})

		var verifiedLSAR bool
		if err == nil && lsarAllowed {
			verifiedLSAR = true
			framework.Logf("api call by %q was successful", saName)
		}

		status, ok = err.(*apierrors.StatusError)
		if ok && status.ErrStatus.Code == 403 && !sarAllowed {
			verifiedLSAR = true
			framework.Logf("api call by %q was denied", saName)
		}

		if verifiedLSAR {
			framework.Logf("LocalSubjectAccessReview has been verified")
		} else {
			framework.Fail(fmt.Sprintf("Could not verify LocalSubjectAccessReview for %q in namespace %q", saName, ns))
		}
	})
})
