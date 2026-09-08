/*
Copyright (c) 2026 Oracle and/or its affiliates.

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

// Package envtest provides a reusable Kubernetes API server and controller
// manager for CAPOCI integration tests.
package envtest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerenvtest "sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	startupTimeout  = 30 * time.Second
	shutdownTimeout = 10 * time.Second
)

// SetupManagerFunc registers controllers and webhooks with an envtest manager.
type SetupManagerFunc func(context.Context, manager.Manager) error

// Options configures an Environment.
type Options struct {
	Scheme               *runtime.Scheme
	CRDDirectoryPaths    []string
	WebhookManifestPaths []string
	SetupManager         SetupManagerFunc
}

// Environment encapsulates a local Kubernetes API server and a running
// controller-runtime manager.
type Environment struct {
	manager.Manager
	client.Client
	Config *rest.Config

	controlPlane *controllerenvtest.Environment
	cancel       context.CancelFunc
	managerDone  chan error
}

// Start creates the API server, installs CRDs and webhook configurations,
// registers the requested manager components, and starts the manager.
func Start(ctx context.Context, options Options) (*Environment, error) {
	if options.Scheme == nil {
		return nil, errors.New("envtest scheme is required")
	}

	controlPlane := &controllerenvtest.Environment{
		Scheme:                options.Scheme,
		CRDDirectoryPaths:     options.CRDDirectoryPaths,
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: controllerenvtest.WebhookInstallOptions{
			Paths:            options.WebhookManifestPaths,
			LocalServingHost: "127.0.0.1",
		},
	}

	config, err := controlPlane.Start()
	if err != nil {
		return nil, fmt.Errorf("start envtest control plane: %w", err)
	}

	webhookOptions := controlPlane.WebhookInstallOptions
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: options.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
		PprofBindAddress:       "0",
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookOptions.LocalServingHost,
			Port:    webhookOptions.LocalServingPort,
			CertDir: webhookOptions.LocalServingCertDir,
		}),
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create envtest manager: %w", err), controlPlane.Stop())
	}

	if options.SetupManager != nil {
		if err := options.SetupManager(ctx, mgr); err != nil {
			return nil, errors.Join(fmt.Errorf("set up envtest manager: %w", err), controlPlane.Stop())
		}
	}

	managerContext, cancel := context.WithCancel(ctx)
	environment := &Environment{
		Manager:      mgr,
		Client:       mgr.GetClient(),
		Config:       config,
		controlPlane: controlPlane,
		cancel:       cancel,
		managerDone:  make(chan error, 1),
	}

	go func() {
		environment.managerDone <- mgr.Start(managerContext)
		close(environment.managerDone)
	}()

	startupContext, startupCancel := context.WithTimeout(ctx, startupTimeout)
	defer startupCancel()

	select {
	case <-mgr.Elected():
	case err := <-environment.managerDone:
		if err == nil {
			err = errors.New("envtest manager stopped without an error")
		}
		return nil, errors.Join(fmt.Errorf("envtest manager stopped during startup: %w", err), controlPlane.Stop())
	case <-startupContext.Done():
		return nil, errors.Join(fmt.Errorf("wait for envtest manager startup: %w", startupContext.Err()), environment.Stop())
	}

	if !mgr.GetCache().WaitForCacheSync(startupContext) {
		return nil, errors.Join(errors.New("envtest manager cache did not sync"), environment.Stop())
	}

	if len(options.WebhookManifestPaths) > 0 {
		webhookStarted := mgr.GetWebhookServer().StartedChecker()
		if err := wait.PollUntilContextCancel(startupContext, 50*time.Millisecond, true, func(context.Context) (bool, error) {
			return webhookStarted(&http.Request{}) == nil, nil
		}); err != nil {
			return nil, errors.Join(fmt.Errorf("wait for envtest webhook server: %w", err), environment.Stop())
		}
	}

	return environment, nil
}

// Stop shuts down the manager and local API server.
func (e *Environment) Stop() error {
	if e == nil {
		return nil
	}

	e.cancel()

	var managerErr error
	select {
	case managerErr = <-e.managerDone:
		if errors.Is(managerErr, context.Canceled) {
			managerErr = nil
		}
	case <-time.After(shutdownTimeout):
		managerErr = errors.New("timed out waiting for envtest manager to stop")
	}

	return errors.Join(managerErr, e.controlPlane.Stop())
}
