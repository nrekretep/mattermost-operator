package clusterinstallation

import (
	"testing"
	"time"

	mattermostv1alpha1 "github.com/mattermost/mattermost-operator/pkg/apis/mattermost/v1alpha1"
	mysqlOperator "github.com/oracle/mysql-operator/pkg/apis/mysql/v1alpha1"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var depKey = types.NamespacedName{Name: "foo", Namespace: "default"}
var depIngressKey = types.NamespacedName{Name: "foo-ingress", Namespace: "default"}
var depMysqlKey = types.NamespacedName{Name: "foo-mysql", Namespace: "default"}
var depSvcAccountKey = types.NamespacedName{Name: "mysql-agent", Namespace: "default"}

const timeout = time.Second * 60

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	instance := &mattermostv1alpha1.ClusterInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: mattermostv1alpha1.ClusterInstallationSpec{
			IngressName: "foo.mattermost.dev",
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create the ClusterInstallation object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// Mysql test section
	mysql := &mysqlOperator.Cluster{}
	g.Eventually(func() error { return c.Get(context.TODO(), depMysqlKey, mysql) }, timeout).
		Should(gomega.Succeed())

	svcAccount := &corev1.ServiceAccount{}
	g.Eventually(func() error { return c.Get(context.TODO(), depSvcAccountKey, svcAccount) }, timeout).
		Should(gomega.Succeed())

	roleBinding := &rbacv1beta1.RoleBinding{}
	g.Eventually(func() error { return c.Get(context.TODO(), depSvcAccountKey, roleBinding) }, timeout).
		Should(gomega.Succeed())

	// Mattermost test section
	service := &corev1.Service{}
	g.Eventually(func() error { return c.Get(context.TODO(), depKey, service) }, timeout).
		Should(gomega.Succeed())

	ingress := &v1beta1.Ingress{}
	g.Eventually(func() error { return c.Get(context.TODO(), depIngressKey, ingress) }, timeout).
		Should(gomega.Succeed())

	// Create the mysql secret
	dbSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-mysql-root-password",
			Namespace: instance.Namespace,
		},
		Data: map[string][]byte{
			"password": []byte("mysupersecure"),
		},
	}
	err = c.Create(context.TODO(), dbSecret)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	defer c.Delete(context.TODO(), dbSecret)

	deploy := &appsv1.Deployment{}
	g.Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
		Should(gomega.Succeed())

}
