package kube

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
)

const (
	defaultScanTimeout   = 30 * time.Second
	defaultEventsTimeout = 30 * time.Second
	highRestartThreshold = 5
	maxEventsReturned    = 20
)

type PodIssue struct {
	Context   string
	Namespace string
	PodName   string
	Status    string
	Reason    string
	Restarts  int32
	Age       time.Duration
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

func ScanAll(clients []ContextClient, opts ScanOptions) ([]PodIssue, error) {
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
		issues []PodIssue
	)

	for _, cc := range clients {
		wg.Add(1)
		go func(cc ContextClient) {
			defer wg.Done()
			found, err := scanContext(cc.Name, cc.Client, opts.Namespace, podRe)
			if err != nil {
				mu.Lock()
				issues = append(issues, PodIssue{
					Context:   cc.Name,
					Namespace: "-",
					PodName:   "-",
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
		return issues[i].PodName < issues[j].PodName
	})

	return issues, nil
}

func scanContext(contextName string, client kubernetes.Interface, namespace string, podRe *regexp.Regexp) ([]PodIssue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultScanTimeout)
	defer cancel()

	podList, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var issues []PodIssue
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

func inspectPod(contextName string, pod *corev1.Pod) *PodIssue {
	age := time.Since(pod.CreationTimestamp.Time)
	phase := pod.Status.Phase

	if phase == corev1.PodSucceeded {
		return nil
	}

	if phase == corev1.PodFailed {
		return &PodIssue{
			Context:   contextName,
			Namespace: pod.Namespace,
			PodName:   pod.Name,
			Status:    StatusFailed,
			Reason:    pod.Status.Reason,
			Age:       age,
		}
	}

	if phase == corev1.PodUnknown {
		return &PodIssue{
			Context:   contextName,
			Namespace: pod.Namespace,
			PodName:   pod.Name,
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

func inspectContainerStatus(contextName string, pod *corev1.Pod, cs *corev1.ContainerStatus, age time.Duration) *PodIssue {
	var status, reason string
	var restarts int32 = cs.RestartCount

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

	return &PodIssue{
		Context:   contextName,
		Namespace: pod.Namespace,
		PodName:   pod.Name,
		Status:    status,
		Reason:    reason,
		Restarts:  restarts,
		Age:       age,
	}
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
		age := time.Since(e.LastTimestamp.Time)
		if age < 0 {
			age = 0
		}
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
