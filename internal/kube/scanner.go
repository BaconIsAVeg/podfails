package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	KindPod = "Pod"
	KindHPA = "HPA"
)

const (
	StatusCrashLoopBackOff           = "CrashLoopBackOff"
	StatusOOMKilled                  = "OOMKilled"
	StatusError                      = "Error"
	StatusFailed                     = "Failed"
	StatusScanError                  = "ScanError"
	StatusImagePullBackOff           = "ImagePullBackOff"
	StatusErrImagePull               = "ErrImagePull"
	StatusCreateContainerConfigError = "CreateContainerConfigError"
	StatusRunContainerError          = "RunContainerError"
	StatusPending                    = "Pending"
	StatusHighRestarts               = "HighRestarts"
	StatusUnknown                    = "Unknown"

	StatusAtMaxReplicas  = "AtMaxReplicas"
	StatusScalingLimited = "ScalingLimited"
)

const (
	defaultScanTimeout    = 30 * time.Second
	defaultHPAScanTimeout = 30 * time.Second
	defaultEventsTimeout  = 30 * time.Second
	highRestartThreshold  = 5
	maxEventsReturned     = 20
)

type Issue struct {
	Kind            string
	Context         string
	Namespace       string
	Name            string
	Status          string
	Reason          string
	Metric          string
	Age             time.Duration
	Conditions      []Condition
	MinReplicas     int32
	MaxReplicas     int32
	CurrentReplicas int32
}

type jsonIssue struct {
	Kind            string          `json:"kind"`
	Context         string          `json:"context"`
	Namespace       string          `json:"namespace"`
	Name            string          `json:"name"`
	Status          string          `json:"status"`
	Reason          string          `json:"reason,omitempty"`
	Metric          string          `json:"metric"`
	Age             string          `json:"age"`
	Conditions      []jsonCondition `json:"conditions,omitempty"`
	MinReplicas     int32           `json:"min_replicas,omitempty"`
	MaxReplicas     int32           `json:"max_replicas,omitempty"`
	CurrentReplicas int32           `json:"current_replicas,omitempty"`
}

func (i Issue) MarshalJSON() ([]byte, error) {
	conditions := make([]jsonCondition, len(i.Conditions))
	for j, c := range i.Conditions {
		conditions[j] = jsonCondition{
			Type:    c.Type,
			Status:  c.Status,
			Reason:  c.Reason,
			Message: c.Message,
			Age:     FormatAge(c.Age),
		}
	}
	ji := jsonIssue{
		Kind:            i.Kind,
		Context:         i.Context,
		Namespace:       i.Namespace,
		Name:            i.Name,
		Status:          i.Status,
		Reason:          i.Reason,
		Metric:          i.Metric,
		Age:             FormatAge(i.Age),
		Conditions:      conditions,
		MinReplicas:     i.MinReplicas,
		MaxReplicas:     i.MaxReplicas,
		CurrentReplicas: i.CurrentReplicas,
	}
	return json.Marshal(ji)
}

type Condition struct {
	Type    string
	Status  string
	Reason  string
	Message string
	Age     time.Duration
}

type jsonCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Age     string `json:"age"`
}

type Event struct {
	Type    string
	Reason  string
	Message string
	Count   int32
	Age     time.Duration
}

var troubledWaitingReasons = map[string]bool{
	StatusCrashLoopBackOff:           true,
	StatusImagePullBackOff:           true,
	StatusErrImagePull:               true,
	StatusCreateContainerConfigError: true,
	StatusRunContainerError:          true,
	StatusError:                      true,
}

type ScanOptions struct {
	PodRegex  string
	Namespace string
}

func ScanAll(clients []ContextClient, opts ScanOptions) ([]Issue, error) {
	var podRe *regexp.Regexp
	if opts.PodRegex != "" {
		var err error
		podRe, err = regexp.Compile(opts.PodRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid pod regex %q: %w", opts.PodRegex, err)
		}
	}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		issues []Issue
	)

	for _, cc := range clients {
		wg.Add(1)
		go func(cc ContextClient) {
			defer wg.Done()
			found, err := scanPods(cc.Name, cc.Client, opts.Namespace, podRe)
			if err != nil {
				mu.Lock()
				issues = append(issues, Issue{
					Kind:      KindPod,
					Context:   cc.Name,
					Namespace: "-",
					Name:      "-",
					Status:    StatusScanError,
					Reason:    err.Error(),
				})
				mu.Unlock()
				return
			}
			mu.Lock()
			issues = append(issues, found...)
			mu.Unlock()
		}(cc)
	}

	for _, cc := range clients {
		wg.Add(1)
		go func(cc ContextClient) {
			defer wg.Done()
			found, err := scanHPAs(cc.Name, cc.Client, opts.Namespace, podRe)
			if err != nil {
				mu.Lock()
				issues = append(issues, Issue{
					Kind:      KindHPA,
					Context:   cc.Name,
					Namespace: "-",
					Name:      "-",
					Status:    StatusScanError,
					Reason:    err.Error(),
				})
				mu.Unlock()
				return
			}
			mu.Lock()
			issues = append(issues, found...)
			mu.Unlock()
		}(cc)
	}

	wg.Wait()

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Context != issues[j].Context {
			return issues[i].Context < issues[j].Context
		}
		if issues[i].Namespace != issues[j].Namespace {
			return issues[i].Namespace < issues[j].Namespace
		}
		return issues[i].Name < issues[j].Name
	})

	return issues, nil
}

func scanPods(contextName string, client kubernetes.Interface, namespace string, podRe *regexp.Regexp) ([]Issue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultScanTimeout)
	defer cancel()

	podList, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var issues []Issue
	for _, pod := range podList.Items {
		if podRe != nil && !podRe.MatchString(pod.Name) {
			continue
		}
		issue := inspectPod(contextName, &pod)
		if issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues, nil
}

func inspectPod(contextName string, pod *corev1.Pod) *Issue {
	age := time.Since(pod.CreationTimestamp.Time)
	phase := pod.Status.Phase

	if phase == corev1.PodSucceeded {
		return nil
	}

	if phase == corev1.PodFailed {
		return &Issue{
			Kind:      KindPod,
			Context:   contextName,
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Status:    StatusFailed,
			Reason:    pod.Status.Reason,
			Age:       age,
		}
	}

	if phase == corev1.PodUnknown {
		return &Issue{
			Kind:      KindPod,
			Context:   contextName,
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Status:    StatusUnknown,
			Age:       age,
		}
	}

	allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	for _, cs := range allStatuses {
		if issue := inspectContainerStatus(contextName, pod, &cs, age); issue != nil {
			return issue
		}
	}

	return nil
}

func inspectContainerStatus(contextName string, pod *corev1.Pod, cs *corev1.ContainerStatus, age time.Duration) *Issue {
	var status, reason string
	restarts := cs.RestartCount

	if cs.State.Waiting != nil {
		waitReason := cs.State.Waiting.Reason
		if troubledWaitingReasons[waitReason] {
			status = waitReason
		}
	}

	if cs.LastTerminationState.Terminated != nil {
		reason = cs.LastTerminationState.Terminated.Reason
	}

	if status == "" && restarts > highRestartThreshold {
		status = StatusHighRestarts
	}

	if status == "" {
		return nil
	}

	return &Issue{
		Kind:      KindPod,
		Context:   contextName,
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Status:    status,
		Reason:    reason,
		Metric:    fmt.Sprintf("%d", restarts),
		Age:       age,
	}
}

func scanHPAs(contextName string, client kubernetes.Interface, namespace string, podRe *regexp.Regexp) ([]Issue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultHPAScanTimeout)
	defer cancel()

	hpaList, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing HPAs: %w", err)
	}

	var issues []Issue
	for i := range hpaList.Items {
		hpa := &hpaList.Items[i]
		if podRe != nil && !podRe.MatchString(hpa.Name) {
			continue
		}
		issue := inspectHPA(contextName, hpa)
		if issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues, nil
}

func inspectHPA(contextName string, hpa *autoscalingv2.HorizontalPodAutoscaler) *Issue {
	age := time.Since(hpa.CreationTimestamp.Time)
	currentReplicas := hpa.Status.CurrentReplicas
	maxReplicas := hpa.Spec.MaxReplicas

	metric := fmt.Sprintf("%d/%d", currentReplicas, maxReplicas)

	var conditions []Condition
	for _, cond := range hpa.Status.Conditions {
		condAge := max(time.Since(cond.LastTransitionTime.Time), 0)
		conditions = append(conditions, Condition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
			Age:     condAge,
		})
	}

	if currentReplicas >= maxReplicas && maxReplicas > 1 {
		return &Issue{
			Kind:            KindHPA,
			Context:         contextName,
			Namespace:       hpa.Namespace,
			Name:            hpa.Name,
			Status:          StatusAtMaxReplicas,
			Metric:          metric,
			Age:             age,
			Conditions:      conditions,
			MinReplicas:     derefInt32(hpa.Spec.MinReplicas),
			MaxReplicas:     maxReplicas,
			CurrentReplicas: currentReplicas,
		}
	}

	for _, cond := range hpa.Status.Conditions {
		if cond.Type == autoscalingv2.ScalingLimited && cond.Status == "True" {
			if cond.Reason == "DesiredReplicasBelowMinReplicas" || cond.Reason == "TooFewReplicas" {
				continue
			}
			return &Issue{
				Kind:            KindHPA,
				Context:         contextName,
				Namespace:       hpa.Namespace,
				Name:            hpa.Name,
				Status:          StatusScalingLimited,
				Reason:          cond.Reason,
				Metric:          metric,
				Age:             age,
				Conditions:      conditions,
				MinReplicas:     derefInt32(hpa.Spec.MinReplicas),
				MaxReplicas:     maxReplicas,
				CurrentReplicas: currentReplicas,
			}
		}
	}

	return nil
}

func GetPodEvents(client kubernetes.Interface, namespace, podName string) ([]Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultEventsTimeout)
	defer cancel()

	fieldSelector := fmt.Sprintf(
		"involvedObject.name=%s,involvedObject.namespace=%s,involvedObject.kind=Pod",
		podName, namespace,
	)
	eventList, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching events: %w", err)
	}

	sort.Slice(eventList.Items, func(i, j int) bool {
		ti := eventList.Items[i].LastTimestamp.Time
		tj := eventList.Items[j].LastTimestamp.Time
		return ti.After(tj)
	})

	var events []Event
	for i, e := range eventList.Items {
		if i >= maxEventsReturned {
			break
		}
		age := max(time.Since(e.LastTimestamp.Time), 0)
		events = append(events, Event{
			Type:    e.Type,
			Reason:  e.Reason,
			Message: e.Message,
			Count:   e.Count,
			Age:     age,
		})
	}
	return events, nil
}

func GetHPAEvents(client kubernetes.Interface, namespace, hpaName string) ([]Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultEventsTimeout)
	defer cancel()

	fieldSelector := fmt.Sprintf(
		"involvedObject.name=%s,involvedObject.namespace=%s,involvedObject.kind=HorizontalPodAutoscaler",
		hpaName, namespace,
	)
	eventList, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching HPA events: %w", err)
	}

	sort.Slice(eventList.Items, func(i, j int) bool {
		ti := eventList.Items[i].LastTimestamp.Time
		tj := eventList.Items[j].LastTimestamp.Time
		return ti.After(tj)
	})

	var events []Event
	for i, e := range eventList.Items {
		if i >= maxEventsReturned {
			break
		}
		age := max(time.Since(e.LastTimestamp.Time), 0)
		events = append(events, Event{
			Type:    e.Type,
			Reason:  e.Reason,
			Message: e.Message,
			Count:   e.Count,
			Age:     age,
		})
	}
	return events, nil
}

func FormatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}
