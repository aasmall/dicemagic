package envreader

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/gobuffalo/envy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// EnvReader collects environmental state such as environment variables, kubernetes state, config maps, etc.
type EnvReader struct {
	MissingKeys        [][]string
	Errors             bool
	kubernetesClient   *kubernetes.Clientset
	podInterface       v1.PodInterface
	configMapInterface v1.ConfigMapInterface
	filesystem         IoutilInterface
}

// IoutilInterface enforces the ioutil signature
// useful for testing
type IoutilInterface interface {
	ReadAll(r io.Reader) ([]byte, error)
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
	ReadDir(dirname string) ([]os.FileInfo, error)
	NopCloser(r io.Reader) io.ReadCloser
}
type osioutil struct{}

func (osioutil) ReadAll(r io.Reader) ([]byte, error)           { return ioutil.ReadAll(r) }
func (osioutil) ReadFile(filename string) ([]byte, error)      { return ioutil.ReadFile(filename) }
func (osioutil) ReadDir(dirname string) ([]os.FileInfo, error) { return ioutil.ReadDir(dirname) }
func (osioutil) NopCloser(r io.Reader) io.ReadCloser           { return ioutil.NopCloser(r) }
func (osioutil) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}

// ReaderOption defines a function to set ReaderOptions
type ReaderOption func(*ReaderOptions)

// ReaderOptions define the behavior of a new reader
type ReaderOptions struct {
	PodInterface       v1.PodInterface
	ConfigMapInterface v1.ConfigMapInterface
	FileSystem         IoutilInterface
}

// WithPodInterface sets the Kubernetes Pod Interface for the reader.
// Useful for testing
func WithPodInterface(podInterface v1.PodInterface) ReaderOption {
	return func(o *ReaderOptions) {
		o.PodInterface = podInterface
	}
}

// WithConfigMapInterface sets the Kubernetes ConfigMap Interface for the reader.
// Useful for testing
func WithConfigMapInterface(configMapInterface v1.ConfigMapInterface) ReaderOption {
	return func(o *ReaderOptions) {
		o.ConfigMapInterface = configMapInterface
	}
}

// WithFilesystem sets the filesystem Interface for the reader.
// Useful for testing
func WithFilesystem(fs IoutilInterface) ReaderOption {
	return func(o *ReaderOptions) {
		o.FileSystem = fs
	}
}

// NewEnvReader returns a new EnvReader with options specified
func NewEnvReader(options ...ReaderOption) *EnvReader {
	readerOptions := ReaderOptions{}
	for _, o := range options {
		o(&readerOptions)
	}
	retReader := &EnvReader{
		podInterface:       readerOptions.PodInterface,
		configMapInterface: readerOptions.ConfigMapInterface,
		filesystem:         readerOptions.FileSystem,
	}
	if readerOptions.FileSystem == nil {
		retReader.filesystem = osioutil{}
	}
	return retReader
}

// GetEnv returns an envrironment variable
func (r *EnvReader) GetEnv(key string) string {
	if value, err := envy.MustGet(key); err == nil {
		return value
	}
	r.Errors = true
	r.MissingKeys = append(r.MissingKeys, []string{key, ""})
	return ""
}

// GetFromFile returns the content of a file at path
func (r *EnvReader) GetFromFile(path string) []byte {
	content, err := r.filesystem.ReadFile(path)
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
	configMapInterface := r.configMapInterface
	if configMapInterface == nil {
		if r.kubernetesClient == nil {
			err := r.getKubernetesClient()
			if err != nil {
				r.Errors = true
				r.MissingKeys = append(r.MissingKeys, []string{strConfigMap, err.Error()})
				return ""
			}
		}
		configMapInterface = r.kubernetesClient.CoreV1().ConfigMaps(namespace)
	}
	configMap, err := configMapInterface.Get(context.TODO(), configMapName, metav1.GetOptions{})
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

// GetPodHosts returns the host names for all pods that match the label selector.
func (r *EnvReader) GetPodHosts(namespace string, labelSelector string) []string {
	strPosHosts := fmt.Sprintf("PodHosts: %s.%s", namespace, labelSelector)
	podInterface := r.podInterface
	if podInterface == nil {
		if r.kubernetesClient == nil {
			err := r.getKubernetesClient()
			if err != nil {
				r.Errors = true
				r.MissingKeys = append(r.MissingKeys, []string{strPosHosts, err.Error()})
				return nil
			}
		}
		podInterface = r.kubernetesClient.CoreV1().Pods(namespace)
	}
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	pods, err := podInterface.List(context.TODO(), listOptions)
	if err != nil {
		r.Errors = true
		r.MissingKeys = append(r.MissingKeys, []string{strPosHosts, err.Error()})
		return nil
	}
	var hosts []string
	for i := 0; i < len(pods.Items); i++ {
		hosts = append(hosts, pods.Items[i].Status.PodIP)
	}
	return hosts
}

// GetEnvOpt returns an envrironment variable, but does not return an error if the variable does not exist.
func (r *EnvReader) GetEnvOpt(key string) string {
	if value, err := envy.MustGet(key); err == nil {
		return value
	}
	return ""
}

// GetEnvBool returns the value of an environment variable as a bool using strconv.ParseBool. returns false if not found.
func (r *EnvReader) GetEnvBool(key string) bool {
	text := r.GetEnvOpt(key)
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
