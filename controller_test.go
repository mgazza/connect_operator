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

/*
import (
	goerrros "errors"
	"reflect"
	"testing"
	"time"

	_ "k8s.io/client-go/tools/cache"
	_ "k8s.io/client-go/tools/record"

	_ "k8s.io/client-go/tools/cache"
	_ "k8s.io/client-go/tools/record"

	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	_ "k8s.io/client-go/tools/cache"
	_ "k8s.io/client-go/tools/record"

	ksql "connect_operator/pkg/apis/connect_operator/v1alpha1"
	"connect_operator/pkg/generated/clientset/versioned/fake"
	informers "connect_operator/pkg/generated/informers/externalversions"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	client     *fake.Clientset
	kubeclient *k8sfake.Clientset
	// Objects to put in the store.
	fooLister        []*ksql.Connector
	deploymentLister []*apps.Deployment
	// Actions expected to happen on the client.
	kubeactions []core.Action
	actions     []core.Action
	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	objects     []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeobjects = []runtime.Object{}
	return f
}

func newFoo(name string, replicas *int32) *ksql.Connector {
	return &ksql.Connector{
		TypeMeta: metav1.TypeMeta{APIVersion: ksql.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: ksql.ConnectorConfig{},
	}
}

func (f *fixture) newController() (*Controller, informers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	i := informers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	//c := NewController(f.kubeclient, f.client,
	//	k8sI.Apps().V1().Deployments(), i.Ksql().V1alpha1().())

	//c.connectorsSynced = alwaysReady
	//c.deploymentsSynced = alwaysReady
	//c.recorder = &record.FakeRecorder{}

	for _, f := range f.fooLister {
		i.Ksql().V1alpha1().ManagedResources().Informer().GetIndexer().Add(f)
	}

	for _, d := range f.deploymentLister {
		k8sI.Apps().V1().Deployments().Informer().GetIndexer().Add(d)
	}

	//return c, i, k8sI
	return nil, nil, nil
}

func (f *fixture) run(fooName string) {
	f.runController(fooName, true, false)
}

func (f *fixture) runExpectError(fooName string) {
	f.runController(fooName, true, true)
}

func (f *fixture) runController(fooName string, startInformers bool, expectError bool) {
	c, i, k8sI := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		i.Start(stopCh)
		k8sI.Start(stopCh)
	}

	err := c.syncHandler(fooName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing foo: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing foo, got nil")
	}

	actions := filterInformerActions(f.client.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}

	k8sActions := filterInformerActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateActionImpl:
		e, _ := expected.(core.CreateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.UpdateActionImpl:
		e, _ := expected.(core.UpdateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.PatchActionImpl:
		e, _ := expected.(core.PatchActionImpl)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expPatch, patch))
		}
	default:
		t.Errorf("Uncaptured Action %s %s, you should explicitly add a case to capture it",
			actual.GetVerb(), actual.GetResource().Resource)
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "foos") ||
				action.Matches("watch", "foos") ||
				action.Matches("list", "deployments") ||
				action.Matches("watch", "deployments")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *fixture) expectCreateDeploymentAction(d *apps.Deployment) {
	f.kubeactions = append(f.kubeactions, core.NewCreateAction(schema.GroupVersionResource{Resource: "deployments"}, d.Namespace, d))
}

func (f *fixture) expectUpdateDeploymentAction(d *apps.Deployment) {
	f.kubeactions = append(f.kubeactions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "deployments"}, d.Namespace, d))
}

func (f *fixture) expectUpdateFooStatusAction(foo *ksql.Connector) {
	action := core.NewUpdateAction(schema.GroupVersionResource{Resource: "foos"}, foo.Namespace, nil)
	// TODO: Until #38113 is merged, we can't use Subresource
	//action.Subresource = "status"
	f.actions = append(f.actions, action)
}

func TestGenerateCreateStatement(t *testing.T) {
	type args struct {
		mr *ksql.Connector
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "When creating a table using column definitions",
			args: args{
				mr: &ksql.Connector{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: ksql.ConnectorConfig{
						Connection: "",
						Name:       "FOO",
						Type:       "TABLE",
						Columns: map[string]string{
							"journey_code": "string PRIMARY KEY",
						},
						With: map[string]string{
							"KAFKA_TOPIC": "'attribution_config'",
						},
						As: "",
					},
					Status: ksql.ConnectorStatus{},
				},
			},
			want: "CREATE TABLE FOO (\njourney_code string PRIMARY KEY) \nWITH (\nKAFKA_TOPIC = 'attribution_config'\n);",
		},
		{
			name: "When creating a table using as",
			args: args{
				mr: &ksql.Connector{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: ksql.ConnectorConfig{
						Connection: "",
						Name:       "FOO",
						Type:       "TABLE",
						As: "SELECT\n        session_id as session_id_key,\n        " +
							"journey_code as journey_code_key,\n        " +
							"attribution_actions['email_click'] as email_click_ts_key,\n        " +
							"attribution_actions['checkout'] as checkout_ts_key,\n        " +
							"AS_VALUE(session_id) as session_id,\n        " +
							"AS_VALUE(journey_code) as journey_code\n    " +
							"FROM SESSION_ACTIONS_ST WINDOW SESSION " +
							"(20 MINUTES, RETENTION 30 MINUTES, GRACE PERIOD 0 SECONDS)\n    " +
							"WHERE\n        attribution_actions['displayed_email'] < attribution_actions['complete_ok'] " +
							"AND\n        attribution_actions['complete_ok'] - attribution_actions['displayed_email'] < 1000000\n    " +
							"GROUP BY journey_code, session_id, attribution_actions['email_click'], attribution_actions['checkout']\n    " +
							"HAVING COUNT(session_id + attribution_actions['displayed_email'] + attribution_actions['complete_ok']) = 1\n",
					},
					Status: ksql.ConnectorStatus{},
				},
			},
			want: "CREATE TABLE FOO AS\n" +
				"SELECT\n        session_id as session_id_key,\n        " +
				"journey_code as journey_code_key,\n        " +
				"attribution_actions['email_click'] as email_click_ts_key,\n        " +
				"attribution_actions['checkout'] as checkout_ts_key,\n        " +
				"AS_VALUE(session_id) as session_id,\n        " +
				"AS_VALUE(journey_code) as journey_code\n    " +
				"FROM SESSION_ACTIONS_ST WINDOW SESSION " +
				"(20 MINUTES, RETENTION 30 MINUTES, GRACE PERIOD 0 SECONDS)\n    " +
				"WHERE\n        attribution_actions['displayed_email'] < attribution_actions['complete_ok'] " +
				"AND\n        attribution_actions['complete_ok'] - attribution_actions['displayed_email'] < 1000000\n    " +
				"GROUP BY journey_code, session_id, attribution_actions['email_click'], attribution_actions['checkout']\n    " +
				"HAVING COUNT(session_id + attribution_actions['displayed_email'] + attribution_actions['complete_ok']) = 1\n;",
		},
		{
			name: "When creating a stream using as",
			args: args{
				mr: &ksql.Connector{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: ksql.ConnectorConfig{
						Connection: "",
						Name:       "FOO",
						Type:       "STREAM",
						As: "SELECT\n    url,\n    journey_code,\n    session_id,\n    actions,\n    EXPLODE(actions) action,\n  " +
							"  as_map(actions, slice(actions, 2, array_length(actions))) AS actions_map,\n    attribution_actions" +
							"\nFROM SESSION_ACTIONS_ST\nEMIT CHANGES",
					},
					Status: ksql.ConnectorStatus{},
				},
			},
			want: "CREATE STREAM FOO AS\n" +
				"SELECT\n    url,\n    journey_code,\n    session_id,\n    actions,\n    EXPLODE(actions) action,\n  " +
				"  as_map(actions, slice(actions, 2, array_length(actions))) AS actions_map,\n    attribution_actions" +
				"\nFROM SESSION_ACTIONS_ST\nEMIT CHANGES;",
		},
		{
			name: "When creating a stream using multiple column definitions",
			args: args{
				mr: &ksql.Connector{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: ksql.ConnectorConfig{
						Connection: "",
						Name:       "FOO",
						Type:       "STREAM",
						Columns: map[string]string{
							"journey_code": "string KEY",
							"session_id":   "string",
						},
						With: map[string]string{
							"KAFKA_TOPIC":  "'attribution_config'",
							"VALUE_FORMAT": "'JSON'",
						},
						As: "",
					},
					Status: ksql.ConnectorStatus{},
				},
			},
			want: "CREATE STREAM FOO (\njourney_code string KEY,\nsession_id string) \nWITH (\nKAFKA_TOPIC = 'attribution_config',\nVALUE_FORMAT = 'JSON'\n);",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateCreateStatement(tt.args.mr); got != tt.want {
				t.Errorf("GenerateCreateStatement() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateManagedResourceSpec(t *testing.T) {
	type args struct {
		mr *ksql.Connector
	}
	tests := []struct {
		name  string
		args  args
		want  error
		want1 string
	}{
		// TODO: Add test cases.
		{
			name: "When only columns are populated",
			args: args{
				mr: &ksql.Connector{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: ksql.ConnectorConfig{
						Connection: "ksql",
						Name:       "name",
						Type:       "TABLE",
						Columns: map[string]string{
							"col": "type",
						},
						With: nil,
						As:   "",
					},
					Status: ksql.ConnectorStatus{},
				},
			},
			want:  goerrros.New(ErrSpecInvalidAsOrColumnsAndWithMustHaveValues),
			want1: MessageSpecInvalidAsOrColumnsAndWithMustHaveValues,
		},
		{
			name: "When only with is populated",
			args: args{
				mr: &ksql.Connector{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: ksql.ConnectorConfig{
						Connection: "ksql",
						Name:       "name",
						Type:       "TABLE",
						Columns:    nil,
						With: map[string]string{
							"col": "type",
						},
						As: "",
					},
					Status: ksql.ConnectorStatus{},
				},
			},
			want:  goerrros.New(ErrSpecInvalidAsOrColumnsAndWithMustHaveValues),
			want1: MessageSpecInvalidAsOrColumnsAndWithMustHaveValues,
		},
		{
			name: "When only as is populated",
			args: args{
				mr: &ksql.Connector{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: ksql.ConnectorConfig{
						Connection: "ksql",
						Name:       "name",
						Type:       "TABLE",
						Columns:    nil,
						With:       nil,
						As:         "blhahalva",
					},
					Status: ksql.ConnectorStatus{},
				},
			},
			want:  nil,
			want1: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ValidateManagedResourceSpec(tt.args.mr)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateManagedResourceSpec() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ValidateManagedResourceSpec() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestExtractQueryID(t *testing.T) {
	type args struct {
		commandMessage string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{
			name: "With success",
			args: args{
				commandMessage: "Created query with ID INSERTQUERY_25",
			},
			want: "INSERTQUERY_25",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractQueryID(tt.args.commandMessage); got != tt.want {
				t.Errorf("ExtractQueryID() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
