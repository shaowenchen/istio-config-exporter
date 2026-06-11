package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseVirtualServiceEntriesDefaultsMissingWeightToZero(t *testing.T) {
	vs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"http": []interface{}{
					map[string]interface{}{
						"match": []interface{}{
							map[string]interface{}{
								"uri": map[string]interface{}{
									"prefix": "/api",
								},
							},
						},
						"route": []interface{}{
							map[string]interface{}{
								"destination": map[string]interface{}{
									"host": "reviews-v1.default.svc.cluster.local",
								},
							},
							map[string]interface{}{
								"destination": map[string]interface{}{
									"host": "reviews-v2.default.svc.cluster.local",
								},
								"weight": int64(25),
							},
						},
					},
				},
			},
		},
	}

	entries := parseVirtualServiceEntries(vs)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].weight != 0 {
		t.Fatalf("expected missing weight to default to 0, got %v", entries[0].weight)
	}
	if entries[1].weight != 25 {
		t.Fatalf("expected explicit weight to be preserved, got %v", entries[1].weight)
	}
}
