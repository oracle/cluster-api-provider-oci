//go:build e2e
// +build e2e

/*
 Copyright (c) 2021, 2022 Oracle and/or its affiliates.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package e2e

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type statefulSetInfo struct {
	name                      string
	namespace                 string
	replicas                  int32
	selector                  map[string]string
	storageClassName          string
	volumeName                string
	svcName                   string
	svcPort                   int32
	svcPortName               string
	containerName             string
	containerImage            string
	containerPort             int32
	podTerminationGracePeriod int64
	volMountPath              string
}

func createLBService(svcNamespace string, svcName string, k8sclient crclient.Client) {
	Byf("Creating service of type Load Balancer with name: %s under namespace: %s", svcName, svcNamespace)
	svcSpec := corev1.ServiceSpec{
		Type: corev1.ServiceTypeLoadBalancer,
		Ports: []corev1.ServicePort{
			{
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			},
		},
		Selector: map[string]string{
			"app": "nginx",
		},
	}
	createService(svcName, svcNamespace, nil, svcSpec, k8sclient)

	Eventually(
		func() (bool, error) {
			svcCreated := &corev1.Service{}
			err := k8sclient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: svcNamespace, Name: svcName}, svcCreated)
			Expect(err).NotTo(HaveOccurred())
			if lbs := len(svcCreated.Status.LoadBalancer.Ingress); lbs > 0 {
				ingressIP := svcCreated.Status.LoadBalancer.Ingress[0].IP
				if ingressIP != "" {
					return true, nil
				}
			}
			return false, nil
		}, 5*time.Minute, 30*time.Second,
	).Should(BeTrue())
}

func createService(svcName string, svcNamespace string, labels map[string]string, serviceSpec corev1.ServiceSpec, k8sClient crclient.Client) {
	svcToCreate := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svcNamespace,
			Name:      svcName,
			Annotations: map[string]string{
				"service.beta.kubernetes.io/oci-load-balancer-internal": "true",
			},
		},
		Spec: serviceSpec,
	}
	if len(labels) > 0 {
		svcToCreate.ObjectMeta.Labels = labels
	}
	Expect(k8sClient.Create(context.TODO(), &svcToCreate)).NotTo(HaveOccurred())
}

func createStatefulSet(statefulsetinfo statefulSetInfo, k8sclient crclient.Client) {
	ginkgo.By("Creating statefulset")
	svcSpec := corev1.ServiceSpec{
		ClusterIP: "None",
		Ports: []corev1.ServicePort{
			{
				Port: statefulsetinfo.svcPort,
				Name: statefulsetinfo.svcPortName,
			},
		},
		Selector: statefulsetinfo.selector,
	}
	createService(statefulsetinfo.svcName, statefulsetinfo.namespace, statefulsetinfo.selector, svcSpec, k8sclient)
	podTemplateSpec := createPodTemplateSpec(statefulsetinfo)
	volClaimTemplate := createPVC(statefulsetinfo)
	deployStatefulSet(statefulsetinfo, volClaimTemplate, podTemplateSpec, k8sclient)
	waitForStatefulSetRunning(statefulsetinfo, k8sclient)
}

func createPVC(statefulsetinfo statefulSetInfo) corev1.PersistentVolumeClaim {
	ginkgo.By("Creating PersistentVolumeClaim config object")
	volClaimTemplate := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: statefulsetinfo.volumeName,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &statefulsetinfo.storageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("50Gi"),
				},
			},
		},
	}
	return volClaimTemplate
}

func deletePVC(info statefulSetInfo, k8sclient crclient.Client) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	err := k8sclient.List(context.TODO(), pvcList, crclient.InNamespace(info.namespace))
	Expect(err).NotTo(HaveOccurred())
	for _, pvc := range pvcList.Items {
		Expect(k8sclient.Delete(context.TODO(), &pvc)).NotTo(HaveOccurred())
		Eventually(
			func() (bool, error) {
				pvcObj := &corev1.PersistentVolumeClaim{}
				err := k8sclient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}, pvcObj)
				if err != nil && errors.IsNotFound(err) {
					return true, nil
				}
				return false, nil
			}, 3*time.Minute, 30*time.Second,
		).Should(BeTrue())
	}

}

func createPodTemplateSpec(statefulsetinfo statefulSetInfo) corev1.PodTemplateSpec {
	ginkgo.By("Creating PodTemplateSpec config object")
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   statefulsetinfo.name,
			Labels: statefulsetinfo.selector,
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &statefulsetinfo.podTerminationGracePeriod,
			Containers: []corev1.Container{
				{
					Name:  statefulsetinfo.containerName,
					Image: statefulsetinfo.containerImage,
					Ports: []corev1.ContainerPort{{Name: statefulsetinfo.svcPortName, ContainerPort: statefulsetinfo.containerPort}},
					VolumeMounts: []corev1.VolumeMount{
						{Name: statefulsetinfo.volumeName, MountPath: statefulsetinfo.volMountPath},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: statefulsetinfo.volumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: statefulsetinfo.volumeName},
					},
				},
			},
		},
	}
	return podTemplateSpec
}

func deployStatefulSet(statefulsetinfo statefulSetInfo, volClaimTemp corev1.PersistentVolumeClaim, podTemplate corev1.PodTemplateSpec, k8sclient crclient.Client) {
	Byf("Deploying Statefulset with name: %s under namespace: %s", statefulsetinfo.name, statefulsetinfo.namespace)
	statefulset := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: statefulsetinfo.name, Namespace: statefulsetinfo.namespace},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:          statefulsetinfo.svcName,
			Replicas:             &statefulsetinfo.replicas,
			Selector:             &metav1.LabelSelector{MatchLabels: statefulsetinfo.selector},
			Template:             podTemplate,
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{volClaimTemp},
		},
	}
	Expect(k8sclient.Create(context.TODO(), &statefulset)).NotTo(HaveOccurred())
}

func deleteStatefulSet(statefulsetinfo statefulSetInfo, k8sclient crclient.Client) {
	Byf("Deploying Statefulset with name: %s under namespace: %s", statefulsetinfo.name, statefulsetinfo.namespace)
	statefulset := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: statefulsetinfo.name, Namespace: statefulsetinfo.namespace},
	}
	Expect(k8sclient.Delete(context.TODO(), &statefulset)).NotTo(HaveOccurred())
	Eventually(
		func() (bool, error) {
			statefulset := &appsv1.StatefulSet{}
			err := k8sclient.Get(context.TODO(), apimachinerytypes.NamespacedName{Name: statefulsetinfo.name, Namespace: statefulsetinfo.namespace}, statefulset)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		}, 3*time.Minute, 30*time.Second,
	).Should(BeTrue())
}

func waitForStatefulSetRunning(info statefulSetInfo, k8sclient crclient.Client) {
	Byf("Ensuring Statefulset(%s) is running", info.name)
	statefulset := &appsv1.StatefulSet{}
	Eventually(
		func() (bool, error) {
			if err := k8sclient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: info.namespace, Name: info.name}, statefulset); err != nil {
				return false, err
			}
			return *statefulset.Spec.Replicas == statefulset.Status.ReadyReplicas, nil
		}, 10*time.Minute, 30*time.Second,
	).Should(BeTrue())
}

func deleteLBService(svcNamespace string, svcName string, k8sclient crclient.Client) {
	svcSpec := corev1.ServiceSpec{
		Type: corev1.ServiceTypeLoadBalancer,
		Ports: []corev1.ServicePort{
			{
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			},
		},
		Selector: map[string]string{
			"app": "nginx",
		},
	}
	deleteService(svcName, svcNamespace, nil, svcSpec, k8sclient)
}

func deleteService(svcName string, svcNamespace string, labels map[string]string, serviceSpec corev1.ServiceSpec, k8sClient crclient.Client) {
	svcToDelete := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svcNamespace,
			Name:      svcName,
		},
		Spec: serviceSpec,
	}
	if len(labels) > 0 {
		svcToDelete.ObjectMeta.Labels = labels
	}
	Expect(k8sClient.Delete(context.TODO(), &svcToDelete)).NotTo(HaveOccurred())
	Eventually(
		func() (bool, error) {
			svcCreated := &corev1.Service{}
			err := k8sClient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: svcNamespace, Name: svcName}, svcCreated)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		}, 3*time.Minute, 30*time.Second,
	).Should(BeTrue())
}
