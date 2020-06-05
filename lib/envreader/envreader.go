package envreader

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// EnvReader collects environmental state such as environment variables, kubernetes state, config maps, etc.
type EnvReader struct {
	MissingKeys      [][]string
	Errors           bool
	kubernetesClient *kubernetes.Clientset
}

// GetEnv returns an envrironment variable
func (r *EnvReader) GetEnv(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	r.Errors = true
	r.MissingKeys = append(r.MissingKeys, []string{key, ""})
	return ""
}

// GetFromFile returns the content of a file at path
func (r *EnvReader) GetFromFile(path string) []byte {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, []string{"file at: " + path, err.Error()})
		return []byte{}
	}
	return content
}

// GetConfigMap returns the current value of a key in a Kubernetes config map
func (r *EnvReader) GetConfigMap(namespace string, configMapName string, dataKey string) string {
	strConfigMap := fmt.Sprintf("config map: %s.%s.%s", namespace, configMapName, dataKey)
	if r.kubernetesClient == nil {
		err := r.getKubernetesClient()
		if err != nil {
			r.Errors = true
			r.MissingKeys = append(r.MissingKeys, []string{strConfigMap, err.Error()})
			return ""
		}
	}
	configMap, err := r.kubernetesClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, []string{strConfigMap, err.Error()})
		return ""
	}
	data := configMap.Data[dataKey]
	if data == "" {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, []string{strConfigMap, err.Error()})
		return ""
	}
	return data
}

// GetPodHosts returns teh host names for all pods that match the label selector.
func (r *EnvReader) GetPodHosts(namespace string, labelSelector string) []string {
	strPosHosts := fmt.Sprintf("PodHosts: %s.%s", namespace, labelSelector)
	if r.kubernetesClient == nil {
		err := r.getKubernetesClient()
		if err != nil {
			r.Errors = true
			r.MissingKeys = append(r.MissingKeys, []string{strPosHosts, err.Error()})
			return nil
		}
	}
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	var hosts []string
	pods, err := r.kubernetesClient.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, []string{strPosHosts, err.Error()})
		return nil
	}
	for i := 0; i < len(pods.Items); i++ {
		hosts = append(hosts, pods.Items[i].Status.PodIP)
	}
	return hosts
}

// GetEnvOpt returns an envrironment variable, but does not return an error if the variable does not exist.
func (r *EnvReader) GetEnvOpt(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return ""
}

// GetEnvBool returns the value of an environment variable as a bool using strconv.ParseBool. returns false if not found.
func (r *EnvReader) GetEnvBool(key string) bool {
	text := r.GetEnv(key)
	if value, err := strconv.ParseBool(text); err == nil {
		return value
	}
	return false
}

// GetEnvFloat returns the value of an environment variable as a float64 using strconv.ParseFloat(text,64)
func (r *EnvReader) GetEnvFloat(key string) float64 {
	text := r.GetEnv(key)

	if value, err := strconv.ParseFloat(text, 64); err != nil {
		return value
	}
	return 0
}

func (r *EnvReader) getKubernetesClient() error {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Could not get Config Map. Error creating Kubernetes InClusterConfig: %s", err)
		return fmt.Errorf("count not create Kubernetes InClusterConfig: %v", err)
	}
	// creates the clientset
	kubernetesClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("could not create Kubernetes client: %v", err)
	}
	r.kubernetesClient = kubernetesClient
	return nil
}
