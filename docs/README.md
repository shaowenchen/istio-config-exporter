# 资源配置说明（导出字段参考）

每个资源一个 YAML 文件，描述典型定义（metadata + spec）。当前 exporter **仅导出** VirtualService 与 DestinationRule 的指定字段，Gateway/ServiceEntry 的 YAML 仅作参考。

| 文件 | 资源 | 是否导出 |
|------|------|----------|
| [virtualservice-spec.yaml](virtualservice-spec.yaml) | VirtualService | 是（uri_prefix/host/weight） |
| [destinationrule-spec.yaml](destinationrule-spec.yaml) | DestinationRule | 是（host + locality distribute） |
| [gateway-spec.yaml](gateway-spec.yaml) | Gateway | 否 |
| [serviceentry-spec.yaml](serviceentry-spec.yaml) | ServiceEntry | 否 |

确定要导出的字段后，可根据上述 YAML 结构修改 collector 的指标与标签。

## 大量 VS/DR 导出建议

集群内 VirtualService、DestinationRule 很多时，可参考 [export-best-practices.md](export-best-practices.md) 控制基数、按命名空间过滤、以及可选汇总指标与 Recording rules 等做法。
