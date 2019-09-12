/*
Copyright 2019 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

package fieldmanager_test

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager"
	"k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/internal"

	"k8s.io/kube-openapi/pkg/util/proto"
	prototesting "k8s.io/kube-openapi/pkg/util/proto/testing"
	"sigs.k8s.io/structured-merge-diff/v3/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v3/merge"
	"sigs.k8s.io/structured-merge-diff/v3/typed"
	"sigs.k8s.io/yaml"
)

var fakeSchema = prototesting.Fake{
	Path: filepath.Join(
		strings.Repeat(".."+string(filepath.Separator), 8),
		"api", "openapi-spec", "swagger.json"),
}

type fakeObjectConvertor struct {
	converter  merge.Converter
	apiVersion fieldpath.APIVersion
}

func (c *fakeObjectConvertor) Convert(in, out, context interface{}) error {
	if typedValue, ok := in.(*typed.TypedValue); ok {
		var err error
		out, err = c.converter.Convert(typedValue, c.apiVersion)
		return err
	}
	out = in
	return nil
}

func (c *fakeObjectConvertor) ConvertToVersion(in runtime.Object, _ runtime.GroupVersioner) (runtime.Object, error) {
	return in, nil
}

func (c *fakeObjectConvertor) ConvertFieldLabel(_ schema.GroupVersionKind, _, _ string) (string, string, error) {
	return "", "", errors.New("not implemented")
}

type fakeObjectDefaulter struct{}

func (d *fakeObjectDefaulter) Default(in runtime.Object) {}

type TestFieldManager struct {
	fieldManager fieldmanager.Manager
	emptyObj     runtime.Object
	liveObj      runtime.Object
}

func NewTestFieldManager(gvk schema.GroupVersionKind) TestFieldManager {
	m := NewFakeOpenAPIModels()
	tc := NewFakeTypeConverter(m)

	converter := internal.NewVersionConverter(tc, &fakeObjectConvertor{}, gvk.GroupVersion())
	apiVersion := fieldpath.APIVersion(gvk.GroupVersion().String())
	f, err := fieldmanager.NewStructuredMergeManager(
		m,
		&fakeObjectConvertor{converter, apiVersion},
		&fakeObjectDefaulter{},
		gv,
		gv,
		true,
	)
	if err != nil {
		panic(err)
	}
	live := &unstructured.Unstructured{}
	live.SetKind(gvk.Kind)
	live.SetAPIVersion(gvk.GroupVersion().String())
	f = fieldmanager.NewStripMetaManager(f)
	f = fieldmanager.NewBuildManagerInfoManager(f, gvk.GroupVersion())
	return TestFieldManager{
		fieldManager: f,
		emptyObj:     live,
		liveObj:      live.DeepCopyObject(),
	}
}

func NewFakeTypeConverter(m proto.Models) internal.TypeConverter {
	tc, err := internal.NewTypeConverter(m, false)
	if err != nil {
		panic(fmt.Sprintf("Failed to build TypeConverter: %v", err))
	}
	return tc
}

func NewFakeOpenAPIModels() proto.Models {
	d, err := fakeSchema.OpenAPISchema()
	if err != nil {
		panic(err)
	}
	m, err := proto.NewOpenAPIData(d)
	if err != nil {
		panic(err)
	}
	return m
}

func (f *TestFieldManager) Reset() {
	f.liveObj = f.emptyObj.DeepCopyObject()
}

func (f *TestFieldManager) Reset() {
	f.liveObj = &unstructured.Unstructured{}
}

func (f *TestFieldManager) Apply(obj []byte, manager string, force bool) error {
	out, err := f.fieldManager.Apply(f.liveObj, obj, manager, force)
	if err == nil {
		f.liveObj = out
	}
	return err
}

func (f *TestFieldManager) Update(obj runtime.Object, manager string) error {
	out, err := f.fieldManager.Update(f.liveObj, obj, manager)
	if err == nil {
		f.liveObj = out
	}
	return err
}

func (f *TestFieldManager) ManagedFields() []metav1.ManagedFieldsEntry {
	accessor, err := meta.Accessor(f.liveObj)
	if err != nil {
		panic(fmt.Errorf("couldn't get accessor: %v", err))
	}

	return accessor.GetManagedFields()
}

// TestUpdateApplyConflict tests that applying to an object, which wasn't created by apply, will give conflicts
func TestUpdateApplyConflict(t *testing.T) {
	f := NewTestFieldManager()

	patch := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "deployment",
			"labels": {"app": "nginx"}
		},
		"spec": {
                        "replicas": 3,
                        "selector": {
                                "matchLabels": {
                                         "app": "nginx"
                                }
                        },
                        "template": {
                                "metadata": {
                                        "labels": {
                                                "app": "nginx"
                                        }
                                },
                                "spec": {
				        "containers": [{
					        "name":  "nginx",
					        "image": "nginx:latest"
				        }]
                                }
                        }
		}
	}`)
	newObj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	if err := yaml.Unmarshal(patch, &newObj.Object); err != nil {
		t.Fatalf("error decoding YAML: %v", err)
	}

	if err := f.Update(newObj, "fieldmanager_test"); err != nil {
		t.Fatalf("failed to apply object: %v", err)
	}

	err := f.Apply([]byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "deployment",
		},
		"spec": {
			"replicas": 101,
		}
	}`), "fieldmanager_conflict", false)
	if err == nil || !apierrors.IsConflict(err) {
		t.Fatalf("Expecting to get conflicts but got %v", err)
	}
}

func TestApplyStripsFields(t *testing.T) {
	f := NewTestFieldManager()

	newObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		},
	}

	newObj.SetName("b")
	newObj.SetNamespace("b")
	newObj.SetUID("b")
	newObj.SetClusterName("b")
	newObj.SetGeneration(0)
	newObj.SetResourceVersion("b")
	newObj.SetCreationTimestamp(metav1.NewTime(time.Now()))
	newObj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    "update",
			Operation:  metav1.ManagedFieldsOperationApply,
			APIVersion: "apps/v1",
		},
	})
	if err := f.Update(newObj, "fieldmanager_test"); err != nil {
		t.Fatalf("failed to apply object: %v", err)
	}

	if m := f.ManagedFields(); len(m) != 0 {
		t.Fatalf("fields did not get stripped: %v", m)
	}
}

func TestVersionCheck(t *testing.T) {
	f := NewTestFieldManager()

	// patch has 'apiVersion: apps/v1' and live version is apps/v1 -> no errors
	err := f.Apply([]byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
	}`), "fieldmanager_test", false)
	if err != nil {
		t.Fatalf("failed to apply object: %v", err)
	}

	// patch has 'apiVersion: apps/v2' but live version is apps/v1 -> error
	err = f.Apply([]byte(`{
		"apiVersion": "apps/v2",
		"kind": "Deployment",
	}`), "fieldmanager_test", false)
	if err == nil {
		t.Fatalf("expected an error from mismatched patch and live versions")
	}
	switch typ := err.(type) {
	default:
		t.Fatalf("expected error to be of type %T was %T", apierrors.StatusError{}, typ)
	case apierrors.APIStatus:
		if typ.Status().Code != http.StatusBadRequest {
			t.Fatalf("expected status code to be %d but was %d",
				http.StatusBadRequest, typ.Status().Code)
		}
	}
}

func TestApplyDoesNotStripLabels(t *testing.T) {
	f := NewTestFieldManager()

	err := f.Apply([]byte(`{
		"apiVersion": "apps/v1",
		"kind": "Pod",
		"metadata": {
			"labels": {
				"a": "b"
			},
		}
	}`), "fieldmanager_test", false)
	if err != nil {
		t.Fatalf("failed to apply object: %v", err)
	}

	if m := f.ManagedFields(); len(m) != 1 {
		t.Fatalf("labels shouldn't get stripped on apply: %v", m)
	}
}

func BenchmarkApplyNewObject(b *testing.B) {
	f := NewTestFieldManager()

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := f.Apply([]byte(`{
		"apiVersion": "apps/v1",
		"kind": "Pod",
		"metadata": {
			"name": "b",
			"namespace": "b",
			"creationTimestamp": "2016-05-19T09:59:00Z",
		},
                "map": {
                        "fieldA": 1,
                        "fieldB": 1,
                        "fieldC": 1,
                        "fieldD": 1,
                        "fieldE": 1,
                        "fieldF": 1,
                        "fieldG": 1,
                        "fieldH": 1,
                        "fieldI": 1,
                        "fieldJ": 1,
                        "fieldK": 1,
                        "fieldL": 1,
                        "fieldM": 1,
                        "fieldN": {
	                        "fieldN": {
					"fieldN": {
						"fieldN": {
				                        "fieldN": {
								"value": true
							},
						},
					},
				},
			},
                }
	}`), "fieldmanager_test", false)
		if err != nil {
			b.Fatal(err)
		}
		f.Reset()
	}
}

func BenchmarkUpdateNewObject(b *testing.B) {
	f := NewTestFieldManager()

	y := `{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "b",
			"namespace": "b",
			"creationTimestamp": "2016-05-19T09:59:00Z",
		},
                "map": {
                        "fieldA": 1,
                        "fieldB": 1,
                        "fieldC": 1,
                        "fieldD": 1,
                        "fieldE": 1,
                        "fieldF": 1,
                        "fieldG": 1,
                        "fieldH": 1,
                        "fieldI": 1,
                        "fieldJ": 1,
                        "fieldK": 1,
                        "fieldL": 1,
                        "fieldM": 1,
                        "fieldN": {
	                        "fieldN": {
					"fieldN": {
						"fieldN": {
				                        "fieldN": {
								"value": true
							},
						},
					},
				},
			},
		},

	}`
	newObj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	if err := yaml.Unmarshal([]byte(y), &newObj.Object); err != nil {
		b.Fatalf("Failed to parse yaml object: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := f.Update(newObj, "fieldmanager_test")
		if err != nil {
			b.Fatal(err)
		}
		f.Reset()
	}
}

func BenchmarkRepeatedUpdate(b *testing.B) {
	f := NewTestFieldManager()

	y1 := `{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "b",
			"namespace": "b",
			"creationTimestamp": "2016-05-19T09:59:00Z",
		},
                "map": {
                        "fieldA": 1,
                        "fieldB": 1,
                        "fieldC": 1,
                        "fieldD": 1,
                        "fieldE": 1,
                        "fieldF": 1,
                        "fieldG": 1,
                        "fieldH": 1,
                        "fieldI": 1,
                        "fieldJ": 1,
                        "fieldK": 1,
                        "fieldL": 1,
                        "fieldM": 1,
                        "fieldN": {
	                        "fieldN": {
					"fieldN": {
						"fieldN": {
				                        "fieldN": {
								"value": true
							},
						},
					},
				},
			},
		},

	}`
	obj1 := &unstructured.Unstructured{Object: map[string]interface{}{}}
	if err := yaml.Unmarshal([]byte(y1), &obj1.Object); err != nil {
		b.Fatalf("Failed to parse yaml object: %v", err)
	}
	y2 := `{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "b",
			"namespace": "b",
			"creationTimestamp": "2016-05-19T09:59:00Z",
		},
                "map": {
                        "fieldA": 1,
                        "fieldB": 1,
                        "fieldC": 1,
                        "fieldD": 1,
                        "fieldE": 1,
                        "fieldF": 1,
                        "fieldG": 1,
                        "fieldH": 1,
                        "fieldI": 1,
                        "fieldJ": 1,
                        "fieldK": 1,
                        "fieldL": 1,
                        "fieldM": 1,
                        "fieldN": {
	                        "fieldN": {
					"fieldN": {
						"fieldN": {
				                        "fieldN": {
								"value": false
							},
						},
					},
				},
			},
		},

	}`
	obj2 := &unstructured.Unstructured{Object: map[string]interface{}{}}
	if err := yaml.Unmarshal([]byte(y2), &obj2.Object); err != nil {
		b.Fatalf("Failed to parse yaml object: %v", err)
	}
	y3 := `{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "b",
			"namespace": "b",
			"creationTimestamp": "2016-05-19T09:59:00Z",
		},
                "map": {
                        "fieldA": 1,
                        "fieldB": 1,
                        "fieldC": 1,
                        "fieldD": 1,
                        "fieldE": 1,
                        "fieldF": 1,
                        "fieldG": 1,
                        "fieldH": 1,
                        "fieldI": 1,
                        "fieldJ": 1,
                        "fieldK": 1,
                        "fieldL": 1,
                        "fieldM": 1,
                        "fieldN": {
	                        "fieldN": {
					"fieldN": {
						"fieldN": {
				                        "fieldN": {
								"value": true
							},
						},
					},
				},
			},
                        "fieldO": 1,
                        "fieldP": 1,
                        "fieldQ": 1,
                        "fieldR": 1,
                        "fieldS": 1,
		},

	}`
	obj3 := &unstructured.Unstructured{Object: map[string]interface{}{}}
	if err := yaml.Unmarshal([]byte(y3), &obj3.Object); err != nil {
		b.Fatalf("Failed to parse yaml object: %v", err)
	}

	objs := []*unstructured.Unstructured{obj1, obj2, obj3}

	if err := f.Update(objs[0], "fieldmanager_0"); err != nil {
		b.Fatal(err)
	}

	if err := f.Update(objs[1], "fieldmanager_1"); err != nil {
		b.Fatal(err)
	}

	if err := f.Update(objs[2], "fieldmanager_2"); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := f.Update(objs[n%3], fmt.Sprintf("fieldmanager_%d", n%3))
		if err != nil {
			b.Fatal(err)
		}
		f.Reset()
	}
}

func TestApplyFailsWithManagedFields(t *testing.T) {
	f := NewTestFieldManager()

	err := f.Apply([]byte(`{
		"apiVersion": "apps/v1",
		"kind": "Pod",
		"metadata": {
			"managedFields": [
				{
				  "manager": "test",
				}
			]
		}
	}`), "fieldmanager_test", false)

	if err == nil {
		t.Fatalf("successfully applied with set managed fields")
	}
}

func TestApplySuccessWithNoManagedFields(t *testing.T) {
	f := NewTestFieldManager()

	err := f.Apply([]byte(`{
		"apiVersion": "apps/v1",
		"kind": "Pod",
		"metadata": {
			"labels": {
				"a": "b"
			},
		}
	}`), "fieldmanager_test", false)

	if err != nil {
		t.Fatalf("failed to apply object: %v", err)
	}
}
