/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	clientSet "connect_operator/pkg/generated/clientset/versioned"
	myInformers "connect_operator/pkg/generated/informers/externalversions"
	"connect_operator/pkg/signals"
	"flag"
	"os"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	masterURL      string
	kubeConfig     string
	connectBaseURL string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err != nil {
		klog.Fatalf("Error building kubeConfig: %s", err.Error())
	}

	kubeClientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientSet: %s", err.Error())
	}

	mgazzaClientSet, err := clientSet.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientSet: %s", err.Error())
	}

	informerFactory := informers.NewSharedInformerFactory(kubeClientSet, time.Second*30)
	mgazzaInformerFactory := myInformers.NewSharedInformerFactory(mgazzaClientSet, time.Second*30)

	connectClient, err := NewConnectClient(connectBaseURL)
	if err != nil {
		klog.Fatalf("Error building connect client %s", err.Error())
	}
	controller := NewController(kubeClientSet,
		mgazzaClientSet,
		mgazzaInformerFactory.Mgazza().V1alpha1().Connectors(),
		connectClient,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	mgazzaInformerFactory.Start(stopCh)
	informerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func init() {
	flag.StringVar(&kubeConfig, "kubeConfig", "", "Path to a kubeConfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeConfig. Only required if out-of-cluster.")
	flag.StringVar(&connectBaseURL, "baseURL", envOrDefault("CONNECT_URL", "http://kafka-connect"), "The Base URL of the kafka connect api")
}
