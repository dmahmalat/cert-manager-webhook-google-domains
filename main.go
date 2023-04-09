package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"go.uber.org/zap"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var GroupName = os.Getenv("GROUP_NAME")
var zapLogger, _ = zap.NewProduction()

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	cmd.RunWebhookServer(GroupName,
		&googledomainsDNSProviderSolver{},
	)
}

type googledomainsDNSProviderSolver struct {
	client *kubernetes.Clientset
}

type googledomainsDNSProviderConfig struct {
	ApiUrl       string `json:"apiUrl"`
	DomainName   string `json:"domainName"`
	SecretRef    string `json:"secretName"`
	SecretKeyRef string `json:"secretKeyName"`
}

type Config struct {
	ApiKey, DomainName, ApiUrl string
}

func (c *googledomainsDNSProviderSolver) Name() string {
	return "google-domains"
}

func (c *googledomainsDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	slogger := zapLogger.Sugar()
	slogger.Infof("call function Present: namespace=%s, zone=%s, fqdn=%s", ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	config, err := clientConfig(c, ch)
	if err != nil {
		return fmt.Errorf("unable to get secret `%s`; %v", ch.ResourceNamespace, err)
	}

	addTxtRecord(config, ch)
	slogger.Infof("Presented txt record %v", ch.ResolvedFQDN)
	return nil
}

func (c *googledomainsDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	slogger := zapLogger.Sugar()
	slogger.Infof("call function CleanUp: namespace=%s, zone=%s, fqdn=%s", ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	config, err := clientConfig(c, ch)
	if err != nil {
		return fmt.Errorf("unable to get secret `%s`; %v", ch.ResourceNamespace, err)
	}

	removeTxtRecord(config, ch)
	slogger.Infof("Cleaned up txt record %v", ch.ResolvedFQDN)
	return nil
}

func (c *googledomainsDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	slogger := zapLogger.Sugar()

	k8sClient, err := kubernetes.NewForConfig(kubeClientConfig)
	slogger.Infof("Input variable stopCh is %d length", len(stopCh))
	if err != nil {
		return err
	}

	c.client = k8sClient
	return nil
}

// Helpers -------------------------------------------------------------------
func addTxtRecord(config Config, ch *v1alpha1.ChallengeRequest) {
	slogger := zapLogger.Sugar()

	url := fmt.Sprintf("%s/acmeChallengeSets/%s:rotateChallenges", config.ApiUrl, config.DomainName)
	var jsonStr = fmt.Sprintf(`
		{
			"accessToken": "%s",
			"keepExpiredRecords": "true",
			"recordsToAdd":
			[
				{
					"digest": "%s",
					"fqdn": "%s"
				}
			]
		}`, config.ApiKey, ch.Key, ch.ResolvedFQDN)

	add, err := callDnsApi(url, "POST", bytes.NewBuffer([]byte(jsonStr)), config)
	if err != nil {
		slogger.Error(err)
	}

	slogger.Infof("Added TXT record result: %s", string(add))
}

func removeTxtRecord(config Config, ch *v1alpha1.ChallengeRequest) {
	slogger := zapLogger.Sugar()

	url := fmt.Sprintf("%s/acmeChallengeSets/%s:rotateChallenges", config.ApiUrl, config.DomainName)
	var jsonStr = fmt.Sprintf(`
		{
			"accessToken": "%s",
			"keepExpiredRecords": "true",
			"recordsToRemove":
			[
				{
					"digest": "%s",
					"fqdn": "%s"
				}
			]
		}`, config.ApiKey, ch.Key, ch.ResolvedFQDN)

	remove, err := callDnsApi(url, "POST", bytes.NewBuffer([]byte(jsonStr)), config)
	if err != nil {
		slogger.Error(err)
	}

	slogger.Infof("Removed TXT record result: %s", string(remove))
}

// Config ------------------------------------------------------
func stringFromSecretData(secretData *map[string][]byte, key string) (string, error) {
	data, ok := (*secretData)[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret data", key)
	}

	return string(data), nil
}

func loadConfig(cfgJSON *extapi.JSON) (googledomainsDNSProviderConfig, error) {
	cfg := googledomainsDNSProviderConfig{}

	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}

	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}
	return cfg, nil
}

func clientConfig(c *googledomainsDNSProviderSolver, ch *v1alpha1.ChallengeRequest) (Config, error) {
	var config Config

	cfg, err := loadConfig((ch.Config))
	if err != nil {
		return config, err
	}

	config.ApiUrl = cfg.ApiUrl
	config.DomainName = cfg.DomainName
	secretName := cfg.SecretRef
	secretKeyName := cfg.SecretKeyRef

	sec, err := c.client.CoreV1().Secrets(ch.ResourceNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return config, fmt.Errorf("unable to get secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err)
	}

	apiKey, err := stringFromSecretData(&sec.Data, secretKeyName)
	config.ApiKey = apiKey
	if err != nil {
		return config, fmt.Errorf("unable to get api-key from secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err)
	}

	return config, nil
}

// REST Request ------------------------------------------------------
func callDnsApi(url string, method string, body io.Reader, config Config) ([]byte, error) {
	slogger := zapLogger.Sugar()

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return []byte{}, fmt.Errorf("unable to execute request %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			slogger.Fatal(err)
		}
	}()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		return respBody, nil
	}

	text := "Error calling API status:" + resp.Status + " url: " + url + " method: " + method
	slogger.Error(text)
	return nil, errors.New(text)
}
