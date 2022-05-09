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

package apps

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/fnv"

	utilrand "k8s.io/apimachinery/pkg/util/rand"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	extensionsinternal "k8s.io/kubernetes/pkg/apis/extensions"
	hashutil "k8s.io/kubernetes/pkg/util/hash"
	labelsutil "k8s.io/kubernetes/pkg/util/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edaemonset "k8s.io/kubernetes/test/e2e/framework/daemonset"
	e2eresource "k8s.io/kubernetes/test/e2e/framework/resource"
	admissionapi "k8s.io/pod-security-admission/api"
)

// This test must be run in serial because it assumes the Daemon Set pods will
// always get scheduled.  If we run other tests in parallel, this may not
// happen.  In the future, running in parallel may work if we have an eviction
// model which lets the DS controller kick out other pods to make room.
// See http://issues.k8s.io/21767 for more details
var _ = SIGDescribe("Controller revision [Serial]", func() {
	var f *framework.Framework

	ginkgo.AfterEach(func() {
		// Clean up
		daemonsets, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
		framework.ExpectNoError(err, "unable to dump DaemonSets")
		if daemonsets != nil && len(daemonsets.Items) > 0 {
			for _, ds := range daemonsets.Items {
				ginkgo.By(fmt.Sprintf("Deleting DaemonSet %q", ds.Name))
				framework.ExpectNoError(e2eresource.DeleteResourceAndWaitForGC(f.ClientSet, extensionsinternal.Kind("DaemonSet"), f.Namespace.Name, ds.Name))
				err = wait.PollImmediate(dsRetryPeriod, dsRetryTimeout, checkRunningOnNoNodes(f, &ds))
				framework.ExpectNoError(err, "error waiting for daemon pod to be reaped")
			}
		}
		if daemonsets, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{}); err == nil {
			framework.Logf("daemonset: %s", runtime.EncodeOrDie(scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...), daemonsets))
		} else {
			framework.Logf("unable to dump daemonsets: %v", err)
		}
		if pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{}); err == nil {
			framework.Logf("pods: %s", runtime.EncodeOrDie(scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...), pods))
		} else {
			framework.Logf("unable to dump pods: %v", err)
		}
		err = clearDaemonSetNodeLabels(f.ClientSet)
		framework.ExpectNoError(err)
	})

	f = framework.NewDefaultFramework("controllerrevisions")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelBaseline

	image := WebserverImage
	dsName := "e2e-" + utilrand.String(5) + "-daemon-set"

	var ns string
	var c clientset.Interface

	ginkgo.BeforeEach(func() {
		ns = f.Namespace.Name

		c = f.ClientSet

		updatedNS, err := patchNamespaceAnnotations(c, ns)
		framework.ExpectNoError(err)

		ns = updatedNS.Name

		err = clearDaemonSetNodeLabels(c)
		framework.ExpectNoError(err)
	})

	ginkgo.It("should test the lifecycle of a ControllerRevision", func() {
		label := map[string]string{daemonsetNameLabel: dsName}
		labelSelector := labels.SelectorFromSet(label).String()

		cs := f.ClientSet

		ginkgo.By(fmt.Sprintf("Creating simple DaemonSet %q", dsName))
		testDaemonset, err := c.AppsV1().DaemonSets(ns).Create(context.TODO(), newDaemonSetWithLabel(dsName, image, label), metav1.CreateOptions{})
		framework.ExpectNoError(err)

		ginkgo.By("Check that daemon pods launch on every node of the cluster.")
		err = wait.PollImmediate(dsRetryPeriod, dsRetryTimeout, checkRunningOnAllNodes(f, testDaemonset))
		framework.ExpectNoError(err, "error waiting for daemon pod to start")
		err = e2edaemonset.CheckDaemonStatus(f, dsName)
		framework.ExpectNoError(err)

		ginkgo.By("listing all DeamonSets")
		dsList, err := cs.AppsV1().DaemonSets("").List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
		framework.ExpectNoError(err, "failed to list Daemon Sets")
		framework.ExpectEqual(len(dsList.Items), 1, "filtered list wasn't found")

		ds, err := c.AppsV1().DaemonSets(ns).Get(context.TODO(), dsName, metav1.GetOptions{})
		framework.ExpectNoError(err)
		framework.Logf("Processing ControllerRevisions for %s", ds.Name)

		ginkgo.By("listing all ControllerRevisions with labelSelector")
		revs, err := cs.AppsV1().ControllerRevisions("").List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
		framework.ExpectNoError(err, "Failed to list ControllerRevision: %v", err)

		// Locate all controller revisions for the current Daemon set
		var revision *appsv1.ControllerRevision
		for _, rev := range revs.Items {
			for _, oref := range rev.OwnerReferences {
				if oref.Kind == "DaemonSet" && oref.UID == ds.UID {
					framework.Logf("revision: %v;hash: %v", rev.Name, rev.ObjectMeta.Labels[appsv1.DefaultDaemonSetUniqueLabelKey])
					revision, err = cs.AppsV1().ControllerRevisions(ns).Get(context.TODO(), rev.Name, metav1.GetOptions{})
					framework.ExpectNoError(err, "failed to lookup ControllerRevision: %v", err)
					framework.ExpectNotEqual(revision, nil, "failed to lookup ControllerRevision: %v", revision)
				}
			}
		}

		ginkgo.By("Create a new ControllerRevision")
		newHash, newName := hashAndNameForDaemonSet(ds)
		newRevision := &appsv1.ControllerRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            newName,
				Namespace:       ds.Namespace,
				Labels:          labelsutil.CloneAndAddLabel(ds.Spec.Template.Labels, appsv1.DefaultDaemonSetUniqueLabelKey, newHash),
				Annotations:     ds.Annotations,
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ds, appsv1.SchemeGroupVersion.WithKind("DaemonSet"))},
			},
			Data:     revision.Data,
			Revision: revision.Revision + 1,
		}
		newControllerRevision, err := cs.AppsV1().ControllerRevisions(ds.Namespace).Create(context.TODO(), newRevision, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create controllerrevision: %v", err)
		framework.Logf("Created ControllerRevision: %v;hash: %v", newControllerRevision.Name, newControllerRevision.ObjectMeta.Labels[appsv1.DefaultDaemonSetUniqueLabelKey])
	})
})

func hashAndNameForDaemonSet(ds *appsv1.DaemonSet) (string, string) {
	hash := fmt.Sprint(ComputeHash(&ds.Spec.Template, ds.Status.CollisionCount))
	name := ds.Name + "-" + hash
	return hash, name
}

func ComputeHash(template *v1.PodTemplateSpec, collisionCount *int32) string {
	podTemplateSpecHasher := fnv.New32a()
	hashutil.DeepHashObject(podTemplateSpecHasher, *template)

	// Add collisionCount in the hash if it exists.
	if collisionCount != nil {
		collisionCountBytes := make([]byte, 8)
		binary.LittleEndian.PutUint32(collisionCountBytes, uint32(*collisionCount))
		podTemplateSpecHasher.Write(collisionCountBytes)
	}

	return rand.SafeEncodeString(fmt.Sprint(podTemplateSpecHasher.Sum32()))
}
