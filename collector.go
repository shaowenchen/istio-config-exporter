package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	istioNetworkingGroup = "networking.istio.io"
	istioNetworkingVer   = "v1beta1"
)

var (
	virtualServiceGVR = schema.GroupVersionResource{
		Group:    istioNetworkingGroup,
		Version:  istioNetworkingVer,
		Resource: "virtualservices",
	}
	destinationRuleGVR = schema.GroupVersionResource{
		Group:    istioNetworkingGroup,
		Version:  istioNetworkingVer,
		Resource: "destinationrules",
	}
)

// VirtualService: one entry per (uri_prefix, host, weight); uri_prefix 为 path 值（来自 uri.prefix / uri.exact / uri.regex）或空
type vsEntry struct {
	uri    string
	host   string
	weight float64 // 指标值 = weight
}

// DestinationRule: one entry per (from, to, weight) from localityLbSetting.distribute
type drEntry struct {
	from   string
	to     string
	weight float64
}

// IstioConfigCollector collects only VirtualService (uri/host/weight) and DestinationRule (locality distribute weight).
type IstioConfigCollector struct {
	mu                    sync.RWMutex
	virtualServices       map[string][]vsEntry // key: namespace/name
	destinationRuleStates map[string]drState   // key: namespace/name

	namespaces []string
	stopCh     chan struct{} // closed on Stop() for graceful shutdown

	virtualServiceSpecDesc  *prometheus.Desc
	destinationRuleSpecDesc *prometheus.Desc
}

type drState struct {
	host    string
	entries []drEntry
}

// Stop stops the informers. Call before process exit for clean shutdown.
func (c *IstioConfigCollector) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

func key(ns, name string) string { return ns + "/" + name }

// sanitizeLabelValue 使标签值符合 Prometheus 规范：不含换行、双引号、反斜杠等，避免 /metrics 报错
func sanitizeLabelValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\n', '\r', '"', '\\':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c *IstioConfigCollector) wantNamespace(ns string) bool {
	if len(c.namespaces) == 0 {
		return true
	}
	for _, n := range c.namespaces {
		if n == ns {
			return true
		}
	}
	return false
}

// NewIstioConfigCollector creates collector and starts Informers for VirtualService and DestinationRule only.
func NewIstioConfigCollector(kubeconfig string, namespacesToScrape []string) (*IstioConfigCollector, error) {
	var config *rest.Config
	var err error
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		}
	}
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})
	c := &IstioConfigCollector{
		virtualServices:       make(map[string][]vsEntry),
		destinationRuleStates: make(map[string]drState),
		namespaces:            namespacesToScrape,
		stopCh:                stopCh,
		virtualServiceSpecDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "virtualservice_spec_uri_host_weight"),
			"VirtualService spec: uri_prefix = /path (from uri.prefix, uri.exact, or uri.regex), destination host; value = route weight",
			[]string{"namespace", "name", "uri_prefix", "host"},
			nil,
		),
		destinationRuleSpecDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "destinationrule_spec_host_trafficpolicy_loadbalancer_localitylbsetting_distribute_weight"),
			"DestinationRule spec: host and trafficPolicy.loadBalancer.localityLbSetting.distribute weight (value=weight)",
			[]string{"namespace", "name", "host", "from", "to"},
			nil,
		),
	}

	factory := dynamicinformer.NewDynamicSharedInformerFactory(client, 0)
	c.registerInformer(factory, virtualServiceGVR, c.handleVirtualService)
	c.registerInformer(factory, destinationRuleGVR, c.handleDestinationRule)

	factory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh,
		factory.ForResource(virtualServiceGVR).Informer().HasSynced,
		factory.ForResource(destinationRuleGVR).Informer().HasSynced) {
		return nil, fmt.Errorf("informer cache sync failed")
	}

	return c, nil
}

func (c *IstioConfigCollector) registerInformer(
	factory dynamicinformer.DynamicSharedInformerFactory,
	gvr schema.GroupVersionResource,
	handler func(obj interface{}, delete bool),
) {
	informer := factory.ForResource(gvr).Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { handler(obj, false) },
		UpdateFunc: func(_, newObj interface{}) { handler(newObj, false) },
		DeleteFunc: func(obj interface{}) { handler(obj, true) },
	})
}

func extractUnstructured(obj interface{}) *unstructured.Unstructured {
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		return o
	case cache.DeletedFinalStateUnknown:
		if u, ok := o.Obj.(*unstructured.Unstructured); ok {
			return u
		}
		return nil
	default:
		return nil
	}
}

func parseVirtualServiceEntries(u *unstructured.Unstructured) []vsEntry {
	var out []vsEntry
	httpSlice, ok, _ := unstructured.NestedSlice(u.Object, "spec", "http")
	if !ok || len(httpSlice) == 0 {
		return out
	}
	for _, h := range httpSlice {
		hm, _ := h.(map[string]interface{})
		if hm == nil {
			continue
		}
		uri := ""
		if matchSlice, ok, _ := unstructured.NestedSlice(hm, "match"); ok && len(matchSlice) > 0 {
			if m, _ := matchSlice[0].(map[string]interface{}); m != nil {
				if uu, ok := m["uri"].(map[string]interface{}); ok {
					if p, _ := uu["prefix"].(string); p != "" {
						uri = p
					} else if e, _ := uu["exact"].(string); e != "" {
						uri = e
					} else if r, _ := uu["regex"].(string); r != "" {
						uri = r
					}
				}
			}
		}
		routeSlice, ok, _ := unstructured.NestedSlice(hm, "route")
		if !ok {
			continue
		}
		for _, r := range routeSlice {
			rm, _ := r.(map[string]interface{})
			if rm == nil {
				continue
			}
			host := ""
			weight := 100.0
			if dest, ok, _ := unstructured.NestedMap(rm, "destination"); ok && dest != nil {
				if h, _ := dest["host"].(string); h != "" {
					host = h
				}
			}
			switch w := rm["weight"].(type) {
			case int64:
				weight = float64(w)
			case int:
				weight = float64(w)
			case float64:
				weight = w
			}
			out = append(out, vsEntry{uri: uri, host: host, weight: weight})
		}
	}
	return out
}

func parseDestinationRuleEntriesWithHost(u *unstructured.Unstructured) (host string, entries []drEntry) {
	host, _, _ = unstructured.NestedString(u.Object, "spec", "host")
	lb, ok, _ := unstructured.NestedMap(u.Object, "spec", "trafficPolicy", "loadBalancer", "localityLbSetting")
	if !ok || lb == nil {
		return host, nil
	}
	distributeSlice, ok, _ := unstructured.NestedSlice(lb, "distribute")
	if !ok || len(distributeSlice) == 0 {
		return host, nil
	}
	for _, d := range distributeSlice {
		dm, _ := d.(map[string]interface{})
		if dm == nil {
			continue
		}
		from, _, _ := unstructured.NestedString(dm, "from")
		toMap, ok, _ := unstructured.NestedMap(dm, "to")
		if !ok || toMap == nil {
			continue
		}
		for toLoc, wVal := range toMap {
			weight := 0.0
			switch v := wVal.(type) {
			case int64:
				weight = float64(v)
			case int:
				weight = float64(v)
			case float64:
				weight = v
			case string:
				if n, err := strconv.ParseInt(v, 10, 64); err == nil {
					weight = float64(n)
				}
			}
			entries = append(entries, drEntry{from: from, to: toLoc, weight: weight})
		}
	}
	return host, entries
}

func (c *IstioConfigCollector) handleVirtualService(obj interface{}, isDelete bool) {
	u := extractUnstructured(obj)
	if u == nil {
		return
	}
	ns, name := u.GetNamespace(), u.GetName()
	k := key(ns, name)
	if isDelete {
		c.mu.Lock()
		delete(c.virtualServices, k)
		c.mu.Unlock()
		return
	}
	if !c.wantNamespace(ns) {
		return
	}
	entries := parseVirtualServiceEntries(u)
	c.mu.Lock()
	if len(entries) > 0 {
		c.virtualServices[k] = entries
	} else {
		delete(c.virtualServices, k)
	}
	c.mu.Unlock()
}

func (c *IstioConfigCollector) handleDestinationRule(obj interface{}, isDelete bool) {
	u := extractUnstructured(obj)
	if u == nil {
		return
	}
	ns, name := u.GetNamespace(), u.GetName()
	k := key(ns, name)
	if isDelete {
		c.mu.Lock()
		delete(c.destinationRuleStates, k)
		c.mu.Unlock()
		return
	}
	if !c.wantNamespace(ns) {
		return
	}
	host, entries := parseDestinationRuleEntriesWithHost(u)
	c.mu.Lock()
	if len(entries) > 0 {
		c.destinationRuleStates[k] = drState{host: host, entries: entries}
	} else {
		delete(c.destinationRuleStates, k)
	}
	c.mu.Unlock()
}

// Describe implements prometheus.Collector.
func (c *IstioConfigCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.virtualServiceSpecDesc
	ch <- c.destinationRuleSpecDesc
}

// Collect implements prometheus.Collector.
func (c *IstioConfigCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	seenVS := make(map[string]struct{})
	for key, entries := range c.virtualServices {
		parts := strings.SplitN(key, "/", 2)
		ns, name := parts[0], ""
		if len(parts) == 2 {
			name = parts[1]
		}
		ns, name = sanitizeLabelValue(ns), sanitizeLabelValue(name)
		for _, e := range entries {
			uriPrefix := sanitizeLabelValue(e.uri)
			host := sanitizeLabelValue(e.host)
			dupKey := ns + "|" + name + "|" + uriPrefix + "|" + host + "|" + strconv.FormatFloat(e.weight, 'f', -1, 64)
			if _, ok := seenVS[dupKey]; ok {
				continue
			}
			seenVS[dupKey] = struct{}{}
			ch <- prometheus.MustNewConstMetric(c.virtualServiceSpecDesc, prometheus.GaugeValue, e.weight, ns, name, uriPrefix, host)
		}
	}

	seenDR := make(map[string]struct{})
	for key, state := range c.destinationRuleStates {
		parts := strings.SplitN(key, "/", 2)
		ns, name := parts[0], ""
		if len(parts) == 2 {
			name = parts[1]
		}
		ns, name = sanitizeLabelValue(ns), sanitizeLabelValue(name)
		host := sanitizeLabelValue(state.host)
		for _, e := range state.entries {
			from, to := sanitizeLabelValue(e.from), sanitizeLabelValue(e.to)
			dupKey := ns + "|" + name + "|" + host + "|" + from + "|" + to
			if _, ok := seenDR[dupKey]; ok {
				continue
			}
			seenDR[dupKey] = struct{}{}
			ch <- prometheus.MustNewConstMetric(c.destinationRuleSpecDesc, prometheus.GaugeValue, e.weight, ns, name, host, from, to)
		}
	}
}
