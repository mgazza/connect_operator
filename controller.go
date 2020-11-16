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
	"context"
	"fmt"
	"time"

	"github.com/go-test/deep"
	errors2 "github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	connectoperatorv1alpha1 "connect_operator/pkg/apis/connect_operator/v1alpha1"

	clientset "connect_operator/pkg/generated/clientset/versioned"
	samplescheme "connect_operator/pkg/generated/clientset/versioned/scheme"
	informers "connect_operator/pkg/generated/informers/externalversions/connect_operator/v1alpha1"
	listers "connect_operator/pkg/generated/listers/connect_operator/v1alpha1"
)

const controllerAgentName = "ksql-manager"

type ConnectClient interface {
	GetConfig(name string) (map[string]interface{}, error)
	Delete(name string) error
	PutConfig(name string, config interface{}) (map[string]interface{}, error)
}

// Controller is the controller implementation for Connector resources
type Controller struct {
	// clientSet is a clientset for our own API group
	clientSet clientset.Interface
	// k8s API group
	kubeclientset kubernetes.Interface

	connectorLister  listers.ConnectorLister
	connectorsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// connectClient is an interface that allows us to connect to the kafka connect api
	connectClient ConnectClient
}

// NewController returns a new sample controller
func NewController(
	kubeClientSet kubernetes.Interface,
	clientSet clientset.Interface,
	connectorInformer informers.ConnectorInformer,
	connectClient ConnectClient,
) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:    kubeClientSet,
		clientSet:        clientSet,
		connectorLister:  connectorInformer.Lister(),
		connectorsSynced: connectorInformer.Informer().HasSynced,
		workqueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Connectors"),
		recorder:         recorder,
		connectClient:    connectClient,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Connector resources change

	connectorInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: controller.enqueueConnector,
			UpdateFunc: func(old, new interface{}) {
				controller.enqueueConnector(new)
			},
			DeleteFunc: controller.enqueueConnector,
		})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Connector controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.connectorsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Connector resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Connector resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Connector resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Connector resource with this namespace/name
	connector, err := c.connectorLister.Connectors(namespace).Get(name)
	if err != nil {
		// The Connector resource may no longer exist if its been deleted
		if errors.IsNotFound(err) {
			err := c.connectClient.Delete(name)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error deleteing connector %s, %s", key, err))
			}
			return nil
		}

		// If an error occurs during Get, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		return err
	}

	config, err := c.buildConnectorConfig(context.Background(), namespace, &connector.Config)
	if err != nil {
		c.setManagedResourceStatus(connector, connectoperatorv1alpha1.ConnectorStatusFailed)
		return nil // return nil dont reprocess
	}

	currentConfig, err := c.connectClient.GetConfig(name)
	if err != nil && errors2.Is(err, ErrNotFound) {
		// lets create it
		_, err := c.connectClient.PutConfig(name, config)
		if err != nil {
			// requeue for processing
			c.setManagedResourceStatus(connector, connectoperatorv1alpha1.ConnectorStatusPending)
			return err
		}

		c.setManagedResourceStatus(connector, connectoperatorv1alpha1.ConnectorStatusApplied)
		return nil

	} else if err != nil {
		c.setManagedResourceStatus(connector, connectoperatorv1alpha1.ConnectorStatusPending)
		return err
	}

	// it exists, but does it match?
	if diff := deep.Equal(currentConfig, config); diff != nil {
		// it has differences so create it
		_, err := c.connectClient.PutConfig(name, config)
		if err != nil {
			// queue for reprocessing
			c.setManagedResourceStatus(connector, connectoperatorv1alpha1.ConnectorStatusPending)
			return err
		}
		c.setManagedResourceStatus(connector, connectoperatorv1alpha1.ConnectorStatusApplied)
	}
	return nil
}

func (c Controller) buildConnectorConfig(ctx context.Context, namespace string, config *connectoperatorv1alpha1.ConnectorConfig) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	var m map[string]connectoperatorv1alpha1.ConfigItem = *config
	for k, v := range m {
		if v.Value != "" {
			result[k] = v.Value
			continue
		}
		if v.ValueFrom == nil {
			continue
		}
		if v.ValueFrom.ConfigMap != nil {
			configMap, err := c.kubeclientset.CoreV1().ConfigMaps(namespace).Get(ctx, v.ValueFrom.ConfigMap.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, errors2.Wrap(err, fmt.Sprintf("ConfigMap refererenced by 'config.%s' was not found", k))
			}
			val, ok := configMap.Data[v.ValueFrom.ConfigMap.Key]
			if !ok {
				return nil, errors2.Wrap(err, fmt.Sprintf("ConfigMap refererenced by 'config.%s' does not contain key %s", k, v.ValueFrom.ConfigMap.Key))
			}
			result[k] = val
			continue
		}
		if v.ValueFrom.Secret != nil {
			secret, err := c.kubeclientset.CoreV1().Secrets(namespace).Get(ctx, v.ValueFrom.Secret.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, errors2.Wrap(err, fmt.Sprintf("Secret refererenced by 'config.%s' was not found", k))
			}
			val, ok := secret.Data[v.ValueFrom.Secret.Key]
			if !ok {
				return nil, errors2.Wrap(err, fmt.Sprintf("Secret refererenced by 'config.%s' does not contain key %s", k, v.ValueFrom.Secret.Key))
			}
			result[k] = string(val)
			continue
		}
	}

	return result, nil
}

// enqueueConnector takes a Connector resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Connector.
func (c *Controller) enqueueConnector(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) setManagedResourceStatus(connector *connectoperatorv1alpha1.Connector, status connectoperatorv1alpha1.ResourceStatus) {
	cp := connector.DeepCopy()
	cp.Status.Applied = status
	err := c.updateConnectorStatus(cp)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error updating status: %v", err))
	}
}

func (c *Controller) updateConnectorStatus(connector *connectoperatorv1alpha1.Connector) error {
	// If the CustomResourceSubResources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Connector resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.clientSet.MgazzaV1alpha1().Connectors(connector.Namespace).
		UpdateStatus(context.Background(), connector, metav1.UpdateOptions{})
	return err
}
