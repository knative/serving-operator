package client

import (
	mf "github.com/manifestival/manifestival"
	"github.com/operator-framework/operator-sdk/pkg/restmapper"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func NewManifest(pathname string, config *rest.Config, opts ...mf.Option) (mf.Manifest, error) {
	client, err := NewClient(config)
	if err != nil {
		return mf.Manifest{}, err
	}
	return mf.NewManifest(pathname, append(opts, mf.UseClient(client))...)
}

func NewClient(config *rest.Config) (mf.Client, error) {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	mapper, err := restmapper.NewDynamicRESTMapper(config)
	if err != nil {
		return nil, err
	}
	return &clientGoClient{client: client, mapper: mapper}, nil
}

type clientGoClient struct {
	client dynamic.Interface
	mapper meta.RESTMapper
}

// verify implementation
var _ mf.Client = (*clientGoClient)(nil)

func (c *clientGoClient) Create(obj *unstructured.Unstructured, options *metav1.CreateOptions) error {
	resource, err := c.resourceInterface(obj)
	if err != nil {
		return err
	}
	_, err = resource.Create(obj, *options)
	return err
}

func (c *clientGoClient) Update(obj *unstructured.Unstructured, options *metav1.UpdateOptions) error {
	resource, err := c.resourceInterface(obj)
	if err != nil {
		return err
	}
	_, err = resource.Update(obj, *options)
	return err
}

func (c *clientGoClient) Delete(obj *unstructured.Unstructured, options *metav1.DeleteOptions) error {
	resource, err := c.resourceInterface(obj)
	if err != nil {
		return err
	}
	return resource.Delete(obj.GetName(), options)
}

func (c *clientGoClient) Get(obj *unstructured.Unstructured, options *metav1.GetOptions) (*unstructured.Unstructured, error) {
	resource, err := c.resourceInterface(obj)
	if err != nil {
		return nil, err
	}
	return resource.Get(obj.GetName(), *options)
}

func (c *clientGoClient) resourceInterface(obj *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	gvk := obj.GroupVersionKind()
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	if mapping.Scope.Name() == meta.RESTScopeNameRoot {
		return c.client.Resource(mapping.Resource), nil
	}
	return c.client.Resource(mapping.Resource).Namespace(obj.GetNamespace()), nil
}
