# 节点筛选、排序与标签

生成的 compact 名称不再追加 D+、GM、CL、SP。仍可显式配置这些平台检测，在结果页查看结果或用于结构化筛选。新增 `telegram` 平台，成功时追加 `TG`。现有配置如果显式列出了 `platforms`，需要自行在列表中加入 `telegram`；不会自动覆盖该列表。

TG 通过当前节点连接 Telegram DC1/DC2 的 TCP 443 端口，发送 MTProto `req_pq_multi`，校验 `resPQ`、随机 nonce 和消息结构；任一数据中心有效响应即通过。检测不需要账号，也不发送登录请求。它表示原生协议初始握手可达，不代表账号登录、所有数据中心、文件下载或语音通话均可用。协议依据：[授权密钥交换](https://core.telegram.org/mtproto/auth_key)、[abridged 传输](https://core.telegram.org/mtproto/mtproto-transports#abridged)。

名称中的 `0%` 仍表示检测接口报告的 IP 风险值，不是丢包率。平台超时或无法判断时不打成功标签，结果页显示未知；未知风险不当作零风险。

## 筛选

推荐用结构化规则表达条件；不同字段必须同时满足，地区/协议列表内部任一匹配即可，平台列表则要求全部通过。例如：

```yaml
media-check: true
platforms: [iprisk, youtube, netflix, openai, telegram]
node-filter:
  regions: [HK, SG]
  protocols: [vless, trojan]
  max-latency: 800
  max-ip-risk: 50
  require-platforms: [openai, telegram]
```

地区优先使用出口查询；查询失败时可从原名称的主体推断，并以 `regionSource: name` 标记，不能视为实测位置。地区不从 `NF-US` 等媒体后缀推断。英国的 UK/GB 统一为 GB。原名称和订阅备注不会代替平台检测结果通过结构化平台规则。

旧 `filter` 保留“多条正则任一匹配”的语义，与结构化规则叠加时必须同时通过。它匹配名称文本，仍可能命中原名或备注中的字样，不适合严格的平台可用性判断。无效正则在配置加载/更新时被拒绝，不会悄悄放行全部节点。原名称 `name-include` / `name-exclude` 和协议规则先执行，以减少无效探测。

测速只接受 HTTP 200/206；默认至少读取 64KB 有效正文，拒绝过小样本和异常断流。可以配置 `speed-min-sample-kb`。开启 `speed-retest` 后，首次有效测速位于最低速度阈值 ±20% 内的节点再测两次，以中位数判定；任一次样本无效则不保留。并发测速仍会共享出口带宽，建议保持较低并发。

## 名额与质量

`success-limit` 始终为硬上限，0 表示不限。`selection.mode` 有三种模式：

- `balanced`：地区均衡，兼容未设置 selection 的旧配置。
- `quality`：全局择优，适合只追求可用质量、不要求地区覆盖。
- `hybrid`：优先地区保底，再结合地区权重和下一候选的质量排名分配剩余名额。

`selection.sort` 可为 `speed`、`latency`、`balanced`。balanced 按 100ms 延迟档位，再按速度比较；speed 优先速度，latency 优先延迟。开启 history 时三种排序都先比较历史存活率档位。最终输出仍按地区分组，组内使用该质量排序；结果页默认保留此顺序，也支持手动排序。

```yaml
selection:
  mode: hybrid
  sort: balanced
  preferred-regions: [HK, SG, JP, US]
  min-per-region: 2
  region-weights: {HK: 2, SG: 2, JP: 1, US: 1}
  max-per-source: 0
  max-per-ip: 0
  history: true
  explore-slots: 2
```

保底名额不足时按优先地区列表轮流分配。hybrid 的 `min-per-region: 0` 使用每地区 1 个的默认保底。来源/IP 上限高于保底优先级；候选不足时可能输出少于 success-limit。一个节点属于多个订阅时，每个来源都计入来源上限。未知出口 IP 不互相合并，也不能保证 IP 多样性。

历史存活率使用实际测活结果，达标节点的速度和延迟采用新值权重 0.3 的指数平滑；30 天未更新的记录清理。至少检测 3 次后，按存活率 ≥95%、≥80%、其余三档排序，新节点处于中间档。`explore-slots` 为观测不足 3 次且本轮已达标的节点优先留出名额（先于地区保底），不会放宽筛选条件。历史保存在私有缓存，不包含原始连接信息。

## 名称与来源

`name-mode: compact`（默认）使用地区序号、速度和检测标签。序号在本轮最终排序后统一生成，只保证本轮一致。

`name-mode: stable` 配合 `rename-node: true` 使用 `SC-<固定ID>`。ID 来自完整连接配置的摘要，不随排序、速度、地区或备注变化；改连接参数会改变 ID。稳定名称不追加动态标签，结果页仍显示测速和平台结果。

去重同样使用完整连接配置，区分协议、WS path、gRPC 等参数；同一节点保留全部订阅来源。订阅统计分别显示“存活/检测、达标、入选”，失效判定只用实际存活结果，避免筛选和限额把健康订阅判死。取消检测不更新订阅健康和质量历史。

来源 URL 不进入公开订阅文件。用于下轮复测的快照保留来源，写入配置文件旁的 `cache/<配置名>/history/`，过期后清理。历史缓存属于干净部署后的私有数据，不再读取旧 `output/history/`。配置重载以默认值加新文件内容重新解析，删除规则即移除规则；无效更新保留上一份配置。
