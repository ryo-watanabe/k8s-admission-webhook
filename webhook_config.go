package main

import (
	"context"
	"sync"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	admregv1 "k8s.io/api/admissionregistration/v1"
	admreg "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

type WebhookConfig struct {
	kubeclient kubernetes.Interface
	client *admreg.AdmissionregistrationV1Client
	wg sync.WaitGroup
}

func newWebhookConfig(kubeconfig string) *WebhookConfig {
	// prepare client
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Exitf("Error building kubeconfig: %s", err.Error())
	}
	kubeclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Exitf("Error building kubernetes clientset: %s", err.Error())
	}
	client, err := admreg.NewForConfig(cfg)
	if err != nil {
		klog.Exitf("Error building admissionregistration clientset: %s", err.Error())
	}
	config := &WebhookConfig{kubeclient: kubeclient, client: client}

	// Prepare ValidatingWebhookConfiguration informer
	informer := informers.NewSharedInformerFactory(config.kubeclient, time.Second*30)
	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { config.Added(obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { config.Updated(newObj) },
		DeleteFunc: func(obj interface{}) { config.Deleted(obj) },
	}
	configInformer := informer.Admissionregistration().V1().ValidatingWebhookConfigurations().Informer()
	configInformer.AddEventHandler(eventHandler)

	// Run informer here instead of call Run() func.
	ctx := context.TODO()
	informer.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), configInformer.HasSynced) {
		klog.Exitf("Error in starting ValidatingWebhookConfigurations informer")
	}
	config.wg.Add(1)

	return config
}

func (c *WebhookConfig) Wait() {
	c.wg.Wait()
}

func (c *WebhookConfig) Added(obj interface{}) {
	// convert object into ValidatingWebhookConfiguration
	vwc, ok := obj.(*admregv1.ValidatingWebhookConfiguration)
	if !ok {
		klog.Warningf("Resource added: Invalid object passed: %#v", obj)
		return
	}
	klog.Infof("VWC %s added", vwc.GetName())
}

func (c *WebhookConfig) Updated(obj interface{}) {
	// convert object into ValidatingWebhookConfiguration
	vwc, ok := obj.(*admregv1.ValidatingWebhookConfiguration)
	if !ok {
		klog.Warningf("Resource updated: Invalid object passed: %#v", obj)
		return
	}
	klog.Infof("VWC %s updated", vwc.GetName())
}

func (c *WebhookConfig) Deleted(obj interface{}) {
	// convert object into ValidatingWebhookConfiguration
	vwc, ok := obj.(*admregv1.ValidatingWebhookConfiguration)
	if !ok {
		klog.Warningf("Resource deleted: Invalid object passed: %#v", obj)
		return
	}
	klog.Infof("VWC %s deleted", vwc.GetName())
}

