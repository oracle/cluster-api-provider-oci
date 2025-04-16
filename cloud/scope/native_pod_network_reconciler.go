/*
 Copyright (c) 2022 Oracle and/or its affiliates.

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

package scope

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	npnVersion          = "oci.oraclecloud.com/v1beta1"
	npnKind             = "NativePodNetwork"
	apiExtensionVersion = "apiextensions.k8s.io/v1"
	npnCrdName          = "nativepodnetworks.oci.oraclecloud.com"
)

func (m *MachineScope) NewWorkloadClient(ctx context.Context) (wlClient client.Client, err error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Secret",
		Version: "v1",
	})
	cluster := m.Cluster

	secret_obj := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name + "-kubeconfig",
	}
	secret := &corev1.Secret{}
	if err := m.Client.Get(ctx, secret_obj, secret); err != nil {
		return nil, err
	}
	secretData := secret.Data["value"]
	config, err := clientcmd.RESTConfigFromKubeConfig(secretData)
	if err != nil {
		m.Info(fmt.Sprintf("error build config: %s", err))
		return nil, err
	}

	wlClient, err = client.New(config, client.Options{})

	return wlClient, err
}

func (m *MachineScope) DeleteNpn(ctx context.Context, wlClient client.Client) error {
	m.Info("DELETE NPN CR NOW.")
	instance, err := m.GetOrCreateMachine(ctx)
	if err != nil {
		m.Info(fmt.Sprintf("Failed to get machine: %s", err))
		return err
	}
	npnCr := &unstructured.Unstructured{}
	slicedId := strings.Split(*instance.Id, ".")
	instanceSuffix := slicedId[len(slicedId)-1]
	npnCr.SetName(instanceSuffix)
	npnCr.SetGroupVersionKind(schema.GroupVersionKind{
		Version: npnVersion,
		Kind:    npnKind,
	})
	if err := wlClient.Delete(ctx, npnCr); err != nil {
		m.Info(fmt.Sprintf("Failed to delete NPN CR: %s", err))
		return err
	}
	return nil
}

func (m *MachineScope) HasNpnCrd(ctx context.Context, wlClient client.Client) (bool, error) {
	m.Info("Get NPN CRD Now.")

	npnCrd := &unstructured.Unstructured{}
	npnCrd.SetGroupVersionKind(schema.GroupVersionKind{
		Version: apiExtensionVersion,
		Kind:    "CustomResourceDefinition",
	})

	err := wlClient.Get(context.Background(), client.ObjectKey{
		Name: npnCrdName,
	}, npnCrd)
	if err != nil {
		m.Info(fmt.Sprintf("Failed to Get NPN CRD, reason: %v", err))
		return false, err
	}

	return true, nil

}

func (m *MachineScope) GetOrCreateNpn(ctx context.Context, wlClient client.Client) (*unstructured.Unstructured, error) {

	m.Info("Get Or Create NPN CR NOW.")
	instance, err := m.GetOrCreateMachine(ctx)
	if err != nil {
		m.Info(fmt.Sprintf("Failed to get machine: %s", err))
		return nil, err
	}

	npnCr := &unstructured.Unstructured{}
	slicedId := strings.Split(*instance.Id, ".")
	instanceSuffix := slicedId[len(slicedId)-1]
	npnCr.SetGroupVersionKind(schema.GroupVersionKind{
		Version: npnVersion,
		Kind:    npnKind,
	})
	err = wlClient.Get(ctx, client.ObjectKey{Name: instanceSuffix}, npnCr)
	// Return NPN CR Object if it existed
	if err == nil {
		m.Info(fmt.Sprintf("Sucessfully Get an Existed NPN CR Object: %s", npnCr))
		return npnCr, nil
	}
	maxPodCount := m.OCIMachine.Spec.MaxPodPerNode
	podSubnetIds := m.OCIMachine.Spec.PodSubnetIds
	podNsgIds := m.OCIMachine.Spec.PodNSGIds
	npnCrCreate := &unstructured.Unstructured{}
	npnCrCreate.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": instanceSuffix,
		},
		"spec": map[string]interface{}{
			"id":                      *instance.Id,
			"maxPodCount":             maxPodCount,
			"podSubnetIds":            podSubnetIds,
			"networkSecurityGroupIds": podNsgIds,
		},
	}

	npnCrCreate.SetGroupVersionKind(schema.GroupVersionKind{
		Version: npnVersion,
		Kind:    npnKind,
	})
	m.Info(fmt.Sprintf("NPN CR to Create is: %v", npnCrCreate))
	err = wlClient.Create(ctx, npnCrCreate)
	return npnCrCreate, err
}
