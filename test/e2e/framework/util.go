package framework

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	kcore "github.com/appscode/kutil/core/v1"
	. "github.com/onsi/ginkgo"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	// apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	myclientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	"github.com/presslabs/mysql-operator/test/e2e/framework/ginkgowrapper"
)

const (
	PodStartTimeout = 5 * time.Minute
	// How often to Poll pods, nodes and claims.
	Poll = 2 * time.Second
)

// CreateTestingNS should be used by every test, note that we append a common prefix to the provided test name.
// Please see NewFramework instead of using this directly.
func CreateTestingNS(baseName string, c clientset.Interface, labels map[string]string) (*v1.Namespace, error) {
	if labels == nil {
		labels = map[string]string{}
	}
	namespaceObj := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// use a short name because long names produce long hostnames but
			// maximum allowed length by mysql is 60.
			// https://dev.mysql.com/doc/refman/8.0/en/change-master-to.html
			GenerateName: fmt.Sprintf("e2e-%v-", baseName),
			Namespace:    "",
			Labels:       labels,
		},
		Status: v1.NamespaceStatus{},
	}
	// Be robust about making the namespace creation call.
	var got *v1.Namespace
	if err := wait.PollImmediate(Poll, 30*time.Second, func() (bool, error) {
		var err error
		got, err = c.CoreV1().Namespaces().Create(namespaceObj)
		if err != nil {
			Logf("Unexpected error while creating namespace: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	Logf("Namespace: %s creeated!", got.Name)
	return got, nil
}

func RestclientConfig(kubeContext string) (*clientcmdapi.Config, error) {
	Logf(">>> kubeConfig: %s", TestContext.KubeConfig)
	if TestContext.KubeConfig == "" {
		return nil, fmt.Errorf("KubeConfig must be specified to load client config")
	}
	c, err := clientcmd.LoadFromFile(TestContext.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("error loading KubeConfig: %v", err.Error())
	}
	if kubeContext != "" {
		Logf(">>> kubeContext: %s", kubeContext)
		c.CurrentContext = kubeContext
	}
	return c, nil
}

func LoadConfig() (*restclient.Config, error) {
	c, err := RestclientConfig(TestContext.KubeContext)
	if err != nil {
		if TestContext.KubeConfig == "" {
			return restclient.InClusterConfig()
		} else {
			return nil, err
		}
	}

	return clientcmd.NewDefaultClientConfig(*c, &clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: TestContext.KubeHost}}).ClientConfig()
}

func KubernetesClients() (clientset.Interface, myclientset.Interface, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, nil, err
	}

	c, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	myC, err := myclientset.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return c, myC, nil
}

func waitTimeoutForPodReadyInNamespace(c clientset.Interface, podName, namespace string, timeout time.Duration) error {
	return wait.PollImmediate(Poll, timeout, podRunningAndReady(c, podName, namespace))
}

func podRunningAndReady(c clientset.Interface, podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := c.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return kcore.PodRunningAndReady(*pod)
	}
}

// deleteNS deletes the provided namespace, waits for it to be completely deleted, and then checks
// whether there are any pods remaining in a non-terminating state.
func DeleteNS(c clientset.Interface, namespace string, timeout time.Duration) error {
	startTime := time.Now()
	if err := c.CoreV1().Namespaces().Delete(namespace, nil); err != nil {
		return err
	}

	// wait for namespace to delete or timeout.
	// err := wait.PollImmediate(2*time.Second, timeout, func() (bool, error) {
	// 	if _, err := c.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{}); err != nil {
	// 		if apierrs.IsNotFound(err) {
	// 			return true, nil
	// 		}
	// 		Logf("Error while waiting for namespace to be terminated: %v", err)
	// 		return false, nil
	// 	}
	// 	return false, nil
	// })

	Logf("namespace %v deletion completed in %s", namespace, time.Now().Sub(startTime))
	return nil
}

func log(level string, format string, args ...interface{}) {
	fmt.Fprintf(GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

func Logf(format string, args ...interface{}) {
	log("INFO", format, args...)
}

func Failf(format string, args ...interface{}) {
	FailfWithOffset(1, format, args...)
}

// FailfWithOffset calls "Fail" and logs the error at "offset" levels above its caller
// (for example, for call chain f -> g -> FailfWithOffset(1, ...) error would be logged for "f").
func FailfWithOffset(offset int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log("INFO", msg)
	ginkgowrapper.Fail(nowStamp()+": "+msg, 1+offset)
}

func Skipf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log("INFO", msg)
	ginkgowrapper.Skip(nowStamp() + ": " + msg)
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func GetPodLogs(c clientset.Interface, namespace, podName, containerName string) (string, error) {
	return getPodLogsInternal(c, namespace, podName, containerName, false)
}

func getPreviousPodLogs(c clientset.Interface, namespace, podName, containerName string) (string, error) {
	return getPodLogsInternal(c, namespace, podName, containerName, true)
}

// utility function for gomega Eventually
func getPodLogsInternal(c clientset.Interface, namespace, podName, containerName string, previous bool) (string, error) {
	logs, err := c.CoreV1().RESTClient().Get().
		Resource("pods").
		Namespace(namespace).
		Name(podName).SubResource("log").
		Param("container", containerName).
		Param("previous", strconv.FormatBool(previous)).
		Do().
		Raw()
	if err != nil {
		return "", err
	}
	if err == nil && strings.Contains(string(logs), "Internal Error") {
		return "", fmt.Errorf("Fetched log contains \"Internal Error\": %q.", string(logs))
	}
	return string(logs), err
}

func kubectlLogPod(c clientset.Interface, pod v1.Pod, containerNameSubstr string, logFunc func(ftm string, args ...interface{})) {
	for _, container := range pod.Spec.Containers {
		if strings.Contains(container.Name, containerNameSubstr) {
			// Contains() matches all strings if substr is empty
			logs, err := GetPodLogs(c, pod.Namespace, pod.Name, container.Name)
			if err != nil {
				logFunc("Failed to get logs of pod %v, container %v, err: %v", pod.Name, container.Name, err)
			}
			plogs, err := getPreviousPodLogs(c, pod.Namespace, pod.Name, container.Name)
			plogs = "PREVIOUS\n" + plogs
			if err != nil {
				plogs = fmt.Sprintf("Failed to get previous logs for pod %v, container %v, err: %v", pod.Name, container.Name, err)
			}
			logFunc("Logs of %v/%v:%v on node %v", pod.Namespace, pod.Name, container.Name, pod.Spec.NodeName)
			logFunc("%s : %s \nSTARTLOG\n%s\nENDLOG for container %v:%v:%v", plogs, containerNameSubstr, logs, pod.Namespace, pod.Name, container.Name)
		}
	}
}

func LogPodsWithLabels(c clientset.Interface, ns string, match map[string]string, logFunc func(ftm string, args ...interface{})) {
	podList, err := c.CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: labels.SelectorFromSet(match).String()})
	if err != nil {
		logFunc("Error getting pods in namespace %q: %v", ns, err)
		return
	}
	logFunc("Running kubectl logs on pods with labels %v in %v", match, ns)
	for _, pod := range podList.Items {
		kubectlLogPod(c, pod, "", logFunc)
	}
}

func LogContainersInPodsWithLabels(c clientset.Interface, ns string, match map[string]string, containerSubstr string, logFunc func(ftm string, args ...interface{})) {
	podList, err := c.CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: labels.SelectorFromSet(match).String()})
	if err != nil {
		Logf("Error getting pods in namespace %q: %v", ns, err)
		return
	}
	for _, pod := range podList.Items {
		kubectlLogPod(c, pod, containerSubstr, logFunc)
	}
}
