# Istio Config Exporter

Prometheus exporter for a subset of Istio networking config: **VirtualService**（spec 中的 uri/host/weight）与 **DestinationRule**（spec 中的 host 与 localityLbSetting.distribute 权重）。

## 功能

- 导出 **VirtualService**：`istio_config_virtualservice_spec_uri_host_weight`（来自 spec.http 的 match uri、route destination host、weight）
- 导出 **DestinationRule**：`istio_config_destinationrule_spec_host_trafficpolicy_loadbalancer_localitylbsetting_distribute_weight`（来自 spec.host 与 trafficPolicy.loadBalancer.localityLbSetting.distribute 的 from/to/weight）

**拉取策略**：使用 Kubernetes Informer（List + Watch）在内存中维护状态；抓取时仅读内存，不对 API Server 发起请求。

## 构建

本项目使用 **vendor** 管理依赖，请先确保已生成并提交 `go.sum` 与 `vendor/`：

```bash
# 若使用 GVM 管理 Go，先切换版本（与 go.mod 中 go 版本一致，如 1.21）
gvm use go1.21

go mod tidy
go mod vendor
# 提交 go.sum 与 vendor/ 后，以下构建均使用本地依赖
make build
```

或使用 Docker（镜像内使用 -mod=vendor，不拉外网）：

```bash
docker build -t shaowenchen/istio-config-exporter:latest .
```

更新依赖时执行：

```bash
go mod tidy
go mod vendor
# 然后提交 go.sum、go.mod、vendor/
```

## 部署

### Kubernetes

需先创建 `monitoring` 命名空间（若不存在），并应用 RBAC 与部署：

```bash
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/servicemonitor.yaml
```

## 使用

```bash
./istio-config-exporter
```

访问 `http://localhost:9102/metrics` 查看指标。

- **健康检查**：`/ready`（就绪）、`/live`（存活），Kubernetes 探针已配置为使用上述路径，避免用 `/metrics` 做健康检查。
- **优雅退出**：收到 SIGTERM/SIGINT 时先停止 Informer 再关闭 HTTP 服务。

命令行参数：

- `-web.listen-address`: 监听地址（默认: `:9102`）
- `-web.telemetry-path`: metrics 路径（默认: `/metrics`）
- `-kubeconfig`: Kubeconfig 路径（可选，集群内可不指定）
- `-namespaces`: 要采集的命名空间，逗号分隔（默认: 空表示全部命名空间）

## 指标（仅此两个）

| 指标名                                                                                                  | 说明                                                           | Labels                                                                 | 指标值   |
| ------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- | --------------------------------------------------------------------- | -------- |
| `istio_config_virtualservice_spec_uri_host_weight`                                                      | VirtualService 中 http 路由的 uri_prefix（path 如 /path）+ host；value=权重 | namespace, name, uri_prefix, host                                     | weight % |
| `istio_config_destinationrule_spec_host_trafficpolicy_loadbalancer_localitylbsetting_distribute_weight` | DestinationRule 的 host 与 locality 分发权重                   | namespace, name, host, from, to                                       | weight % |

### 使用示例

按命名空间统计 VirtualService 路由条数：

```promql
count by (namespace) (istio_config_virtualservice_spec_uri_host_weight)
```

按 host 汇总 DestinationRule 的 locality 权重：

```promql
sum by (host, from, to) (istio_config_destinationrule_spec_host_trafficpolicy_loadbalancer_localitylbsetting_distribute_weight)
```

## Grafana Dashboard

在 Grafana 中导入 `grafana/dashboard.json`。

面板包含两个表格：

- **VirtualService uri / host / weight**：namespace, name, uri_prefix, host, weight
- **DestinationRule locality distribute weight**：namespace, name, host, from, to, value(weight)

变量：`datasource`（Prometheus 数据源）、`namespace`（命名空间筛选，多选）。

## License

MIT
