# 发版说明与版本策略

本目录保存各版本的发布说明。发布流程和版本号规则如下。

## 版本线

- `master`：沿用既有维护线，当前最新为 `v1.6.5`。
- `dev`：新的功能发布线，从 `v3.0.0` 起。仓库中已存在历史上的 `v2.0.0` 至 `v2.3.4` 标签（来自上游），`dev` 线不再复用 `v2.x`，避免覆盖已发布的 tag。

## 版本号规则

- 修复、文档和小范围调整：`v3.0.x`
- 功能新增或默认行为变化：`v3.x.0`
- 不兼容变更：`v4.0.0`

## 发布流程

1. 在 `dev` 上更新 `doc/releases/vX.Y.Z.md`。
2. 提交并推送 `dev`。
3. 在待发布提交上创建并推送 tag：`git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`。
4. `release.yml` 自动构建并推送 GHCR 镜像 `ghcr.io/beihehele/subs-check`。
5. 非 beta 版本同时更新 `latest` 标签。

## 分发范围

- 只发布 GHCR 容器镜像。
- 平台为 `linux/amd64` 和 `linux/arm64`。
- 不发布预编译二进制附件，不发布 Docker Hub 镜像，不支持 `linux/arm/v7`。

## 发布前检查

- `go test ./... -count=1`
- `go vet ./...`
- CI 的 race 测试通过
- CI 的 amd64 / arm64 Docker 构建通过
- `config/config.example.yaml` 复制为 `config.yaml` 后可以直接运行
- 反向代理场景确认 `/admin`、`/static`、`/api` 都能通过同一前缀访问，并覆盖带尾斜杠访问
