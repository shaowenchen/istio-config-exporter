# 资源配置说明（导出字段参考）

每个资源一个 YAML 文件，描述典型定义（metadata + spec）。请在各文件中标注或另行说明需要导出为 Prometheus 指标标签的字段，便于实现 collector。

| 文件 | 资源 |
|------|------|
| [virtualservice-spec.yaml](virtualservice-spec.yaml) | VirtualService |
| [gateway-spec.yaml](gateway-spec.yaml) | Gateway |
| [destinationrule-spec.yaml](destinationrule-spec.yaml) | DestinationRule |
| [serviceentry-spec.yaml](serviceentry-spec.yaml) | ServiceEntry |

确定要导出的字段后，可根据这些 YAML 结构修改 collector 的指标与标签。
