package main

import (
	"context"
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/aasmall/dicemagic/lib/envreader"
	"github.com/davecgh/go-spew/spew"
	"github.com/gobuffalo/envy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type mockPod struct {
	v1.PodInterface
}

func (pi *mockPod) List(ctx context.Context, opts metav1.ListOptions) (*corev1.PodList, error) {
	return &corev1.PodList{Items: []corev1.Pod{{Status: corev1.PodStatus{PodIP: "test_ip"}}}}, nil
}

type mockConfigMap struct {
	v1.ConfigMapInterface
}

func (cmi *mockConfigMap) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.ConfigMap, error) {
	return &corev1.ConfigMap{Data: map[string]string{"test_config_map_key": "test_config_map_value"}}, nil
}

type mockioutil struct {
	envreader.IoutilInterface
}

func (mockioutil) ReadFile(filename string) ([]byte, error) { return []byte("test_file_contents"), nil }

var (
	testConfigMap = &mockConfigMap{}
	testPod       = &mockPod{}
	testioutil    = &mockioutil{}
)

func Test_getEnvironmentalConfig(t *testing.T) {
	envy.Temp(func() {
		tempOSVariables := map[string]string{
			"KMS_KEYRING":                 "test_keyring",
			"KMS_SLACK_KEY":               "test_slack_key",
			"KMS_SLACK_KEY_LOCATION":      "test_slack_key_location",
			"SLACK_CLIENT_ID":             "test_slack_client_id",
			"SLACK_APP_ID":                "test_slack_app_id",
			"SLACK_OAUTH_DENIED_URL":      "test_oauth_denied_url",
			"LOG_NAME":                    "test_log_name",
			"SERVER_PORT":                 "test_server_port",
			"SLACK_TOKEN_URL":             "test_slack_token_url",
			"DICE_SERVER_PORT":            "test_dice_server_port",
			"REDIS_PORT":                  "rest_redis_port",
			"POD_NAME":                    "test_pod_name",
			"REDIRECT_URI":                "test_redirect_uri",
			"SLACK_PROXY_URL":             "test_slack_proxy_uri",
			"MOCK_KMS_URL":                "test_mock_kms_url",
			"GRPC_CLIENT_PORT":            "test_grpc_port",
			"MOCK_DATASTORE_SERVICE_HOST": "test_datastore_service_host",
			"MOCK_DATASTORE_SERVICE_PORT": "test_datastore_service_port",
			"DEBUG":                       "true",
		}
		for key, value := range tempOSVariables {
			envy.Set(key, value)
		}
		errorTest := struct {
			name    string
			want    *envConfig
			wantErr bool
		}{
			name:    "getEnvConfigWithError",
			want:    nil,
			wantErr: true,
		}
		t.Run(errorTest.name, func(t *testing.T) {
			got, err := getEnvironmentalConfig(
				envreader.WithConfigMapInterface(testConfigMap),
				envreader.WithPodInterface(testPod),
				envreader.WithFilesystem(testioutil))
			if (err != nil) != errorTest.wantErr {
				t.Errorf("getEnvironmentalConfig() error = %v, wantErr %v", err, errorTest.wantErr)
				return
			}
			if !reflect.DeepEqual(got, errorTest.want) {
				t.Errorf("getEnvironmentalConfig() = %+v, want %+v", spew.Sdump(got), spew.Sdump(errorTest.want))
			}
		})

		envy.Set("PROJECT_ID", "test_project")
		happyTest := struct {
			name    string
			want    *envConfig
			wantErr bool
		}{
			name: "getEnvConfig",
			want: &envConfig{
				projectID:             "test_project",
				kmsKeyring:            "test_keyring",
				kmsSlackKey:           "test_slack_key",
				kmsSlackKeyLocation:   "test_slack_key_location",
				slackClientID:         "test_slack_client_id",
				slackAppID:            "test_slack_app_id",
				slackOAuthDeniedURL:   "test_oauth_denied_url",
				logName:               "test_log_name",
				serverPort:            "test_server_port",
				slackTokenURL:         "test_slack_token_url",
				diceServerPort:        "test_dice_server_port",
				redisPort:             "rest_redis_port",
				podName:               "test_pod_name",
				localRedirectURI:      "test_redirect_uri",
				slackProxyURL:         "test_slack_proxy_uri",
				mockKMSURL:            "test_mock_kms_url",
				mockDatastoreHost:     "test_datastore_service_host",
				mockDatastorePort:     "test_datastore_service_port",
				grpcClientPort:        "test_grpc_port",
				debug:                 true,
				local:                 false,
				redisClusterHosts:     []string{"test_ip"},
				encSlackSigningSecret: base64.StdEncoding.EncodeToString([]byte("test_file_contents")),
				encSlackClientSecret:  base64.StdEncoding.EncodeToString([]byte("test_file_contents")),
			},
			wantErr: false,
		}
		t.Run(happyTest.name, func(t *testing.T) {
			got, err := getEnvironmentalConfig(
				envreader.WithConfigMapInterface(testConfigMap),
				envreader.WithPodInterface(testPod),
				envreader.WithFilesystem(testioutil))
			if (err != nil) != happyTest.wantErr {
				t.Errorf("getEnvironmentalConfig() error = %v, wantErr %v", err, happyTest.wantErr)
				return
			}
			if !reflect.DeepEqual(got, happyTest.want) {
				t.Errorf("getEnvironmentalConfig() = %+v, want %+v", spew.Sdump(got), spew.Sdump(happyTest.want))
			}
		})
	})
}
