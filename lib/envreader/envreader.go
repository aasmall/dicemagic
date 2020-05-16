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

type EnvReader struct {
	MissingKeys      []string
	Errors           bool
	kubernetesClient *kubernetes.Clientset
}

func (r *EnvReader) GetEnv(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	r.Errors = true
	r.MissingKeys = append(r.MissingKeys, key)
	return ""
}
func (r *EnvReader) GetFromFile(path string) string {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, "file at: "+path)
		return ""
	}
	return string(content)
}
func (r *EnvReader) getKubernetesClient() error {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Could not get Config Map. Error creating Kubernetes InClusterConfig: %s", err)
		return err
	}
	// creates the clientset
	kubernetesClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Could not get Config Map. Error creating Kubernetes Client: %s", err)
		return err
	}
	r.kubernetesClient = kubernetesClient
	return nil
}
func (r *EnvReader) GetConfigMap(namespace string, configMapName string, dataKey string) string {
	if r.kubernetesClient == nil {
		err := r.getKubernetesClient()
		if err != nil {
			r.Errors = true
			r.MissingKeys = append(r.MissingKeys, fmt.Sprintf("%s.%s.%s", namespace, configMapName, dataKey))
			return ""
		}
	}
	configMap, err := r.kubernetesClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, fmt.Sprintf("%s.%s.%s", namespace, configMapName, dataKey))
		return ""
	}
	data := configMap.Data[dataKey]
	if data == "" {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, fmt.Sprintf("%s.%s.%s", namespace, configMapName, dataKey))
		return ""
	}
	return data
}
func (r *EnvReader) GetPodHosts(namespace string, labelSelector string) []string {
	if r.kubernetesClient == nil {
		err := r.getKubernetesClient()
		if err != nil {
			r.Errors = true
			r.MissingKeys = append(r.MissingKeys, fmt.Sprintf("PodHosts: %s.%s", namespace, labelSelector))
			return nil
		}
	}
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	var hosts []string
	pods, err := r.kubernetesClient.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, fmt.Sprintf("PodHosts: %s.%s", namespace, labelSelector))
		return nil
	}
	for i := 0; i < len(pods.Items); i++ {
		hosts = append(hosts, pods.Items[i].Status.PodIP)
	}
	log.Printf("gotPodHosts: %v", hosts)
	return hosts
}
func (r *EnvReader) GetEnvOpt(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return ""
}
func (r *EnvReader) GetEnvBool(key string) bool {
	text := r.GetEnv(key)
	if value, err := strconv.ParseBool(text); err == nil {
		return value
	}
	return false
}
func (r *EnvReader) GetEnvBoolOpt(key string) bool {
	text := r.GetEnvOpt(key)
	if value, err := strconv.ParseBool(text); err == nil {
		return value
	}
	return false
}
func (r *EnvReader) GetEnvFloat(key string) float64 {
	text := r.GetEnv(key)

	if value, err := strconv.ParseFloat(text, 64); err != nil {
		return value
	}
	return 0
}
