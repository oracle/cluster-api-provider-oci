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

package util

import (
	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ClusterUnpausedAndInfrastructureReady returns a Predicate that returns true on Cluster creation events where
// both Cluster.Spec.Paused is false and Cluster.Status.InfrastructureReady is true, and Update events when
// either Cluster.Spec.Paused transitions to false or Cluster.Status.InfrastructureReady transitions to true.
// This is a v1beta1-compatible version of the CAPI predicates.ClusterUnpausedAndInfrastructureProvisioned function.
func ClusterUnpausedAndInfrastructureReady(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := logger.WithValues("predicate", "ClusterUnpausedAndInfrastructureReady", "eventType", "create")
			log = log.WithValues("Cluster", klog.KObj(e.Object))

			c, ok := e.Object.(*clusterv1beta1.Cluster)
			if !ok {
				log.V(4).Info("Expected Cluster", "got", e.Object.GetObjectKind().GroupVersionKind().Kind)
				return false
			}

			// Only process if both not paused and infrastructure is ready
			if !c.Spec.Paused && c.Status.InfrastructureReady {
				log.V(6).Info("Cluster is not paused and infrastructure is ready, allowing further processing")
				return true
			}

			log.V(4).Info("Cluster is paused or infrastructure is not ready, blocking further processing",
				"paused", c.Spec.Paused, "infrastructureReady", c.Status.InfrastructureReady)
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := logger.WithValues("predicate", "ClusterUnpausedAndInfrastructureReady", "eventType", "update")
			log = log.WithValues("Cluster", klog.KObj(e.ObjectOld))

			oldCluster, ok := e.ObjectOld.(*clusterv1beta1.Cluster)
			if !ok {
				log.V(4).Info("Expected Cluster", "got", e.ObjectOld.GetObjectKind().GroupVersionKind().Kind)
				return false
			}

			newCluster, ok := e.ObjectNew.(*clusterv1beta1.Cluster)
			if !ok {
				log.V(4).Info("Expected Cluster", "got", e.ObjectNew.GetObjectKind().GroupVersionKind().Kind)
				return false
			}

			// Process if cluster became unpaused
			if oldCluster.Spec.Paused && !newCluster.Spec.Paused {
				log.V(6).Info("Cluster became unpaused, allowing further processing")
				return true
			}

			// Process if infrastructure became ready
			if !oldCluster.Status.InfrastructureReady && newCluster.Status.InfrastructureReady {
				log.V(6).Info("Cluster infrastructure became ready, allowing further processing")
				return true
			}

			log.V(4).Info("Cluster did not become unpaused or infrastructure did not become ready, blocking further processing")
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
