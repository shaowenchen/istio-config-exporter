# 大量 Istio VS/DR 配置导出：建议与最佳实践

当集群内 VirtualService、DestinationRule 数量很多时，可以按下面方式更优雅、可控地导出指标。

## 1. 控制基数（Cardinality）

- **只采集需要的命名空间**：用 `-namespaces=prod,gateway` 等，避免全集群扫。
- **标签值不要过长**：当前已对 `uri_prefix`、`host` 等做 sanitize；若 regex 非常长，可在 collector 里对长度做截断或只保留类型（如 `regex:<truncated>`），避免单条 label 值过长、序列数爆炸。
- **关注序列总数**：VS 指标序列数 ≈ (VS 数 × 每条 VS 的 route 条数)；DR 序列数 ≈ (DR 数 × 各 DR 的 distribute 条数)。在 Prometheus 里用 `count(istio_config_virtualservice_spec_uri_host_weight)` 等评估，必要时用 `-namespaces` 或后续「汇总指标」做收敛。

## 2. 按场景拆分导出

- **只看概览**：可再起一个“汇总” exporter 或在本 collector 里增加**汇总类指标**（例如每 namespace 的 VS 数量、每 VS 的 route 数量），基数低、适合告警与大盘。
- **看明细**：当前按 uri/host/weight、host/from/to/weight 的明细指标适合排障与审计；建议用 `-namespaces` 只对关键命名空间开明细，其余只看汇总。

## 3. 命名与结构

- **统一用 snake_case 标签名**：如 `uri_prefix`、`host`、`namespace`、`name`，已符合。
- **指标值语义清晰**：VS 用 weight（百分比），DR 用 distribute weight，不混用“1 表示存在”的 info 与“数值表示权重”的 gauge，当前实现已区分。

## 4. 采集与存储

- **Scrape 间隔**：ServiceMonitor 的 `interval` 保持 30s 即可；Informer 已在内存维护状态，抓取成本低。
- **Prometheus 保留与限制**：大量 VS/DR 时建议在 Prometheus 配置里设 `storage.tsdb.retention`、或对 `istio_config_*` 做按需的 limit/drop，避免历史数据撑爆存储。

## 5. 可选增强（按需实现）

| 方向           | 说明 |
|----------------|------|
| **汇总指标**   | 如 `istio_config_virtualservice_routes_total{namespace,name}` = 某 VS 的 route 条数，基数小，便于按 namespace/name 聚合与告警。 |
| **uri 截断**   | 对 `uri` 标签长度做上限（如 128 字符），超长部分用 `...` 或 hash 代替，控制基数与可读性。 |
| **采样/过滤**  | 对非关键 namespace 只导出汇总、不导出每条 uri/host/weight，或通过 `-namespaces` 完全排除。 |
| **Recording rules** | 在 Prometheus 用 recording rules 预聚合（如按 namespace 的 sum、count），查询与告警用预聚合结果，减少高基数查询。 |

## 6. 小结

- 用 **`-namespaces`** 收窄采集范围，是控制基数和成本最直接的方式。
- 保持 **uri_prefix** 单标签（值为 path，如 /path）、**指标值=weight**，不增加空的 uri_exact/uri_regex，便于查询和存储。
- 大量配置时优先保障 **汇总类指标 + 关键 namespace 的明细**，再按需开全量明细并配合 Prometheus 的 retention/limit。
