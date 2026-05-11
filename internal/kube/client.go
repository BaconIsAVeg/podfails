package kube

import (
	"fmt"
	"log"
	"regexp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type ContextClient struct {
	Name   string
	Client kubernetes.Interface
}

func LoadContexts(contextRegex string) ([]ContextClient, error) {
	var re *regexp.Regexp
	if contextRegex != "" {
		var err error
		re, err = regexp.Compile(contextRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid context regex %q: %w", contextRegex, err)
		}
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	rawConfig, err := loadingRules.Load()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	var clients []ContextClient
	for ctxName := range rawConfig.Contexts {
		if re != nil && !re.MatchString(ctxName) {
			continue
		}
		client, err := buildClient(rawConfig, ctxName, loadingRules)
		if err != nil {
			log.Printf("warning: skipping context %q: %v", ctxName, err)
			continue
		}
		clients = append(clients, ContextClient{Name: ctxName, Client: client})
	}

	return clients, nil
}

func buildClient(rawConfig *api.Config, contextName string, loadingRules *clientcmd.ClientConfigLoadingRules) (kubernetes.Interface, error) {
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	clientConfig := clientcmd.NewNonInteractiveClientConfig(
		*rawConfig,
		contextName,
		overrides,
		loadingRules,
	)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config for context %q: %w", contextName, err)
	}
	return kubernetes.NewForConfig(restConfig)
}
