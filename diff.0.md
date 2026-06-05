对比结果如下。`new-minio` 位于 `minio/new-minio/`，是当前项目（`minio-RELEASE.2021-04-22`，Apache 2.0）的后续社区版，两者相差约 5 年演进。

## 1. 许可证（最核心的区别）

| 维度 | 本项目 (minio) | new-minio |
|------|----------------|-----------|
| 协议 | **Apache 2.0** | **GNU AGPL v3** |
| 闭源商用 | 可修改、集成、商用，无强制开源义务 | 通过网络提供服务即触发 AGPL 义务 |
| 合规要求 | 宽松 | 需开源修改后的代码；商用需评估或购买 AIStor 商业许可 |

AGPL 的“传染性”比 GPL 更强：你把 MinIO 作为网络服务跑起来，用户能访问，就必须向用户提供对应源码。闭源产品直接集成或二次封装，风险很高。

本项目 README 写明：

> MinIO is released under **Apache License v2.0**

new-minio README 写明：

> released under the **GNU AGPL v3.0** license  
> All usage requires validation against **AGPLv3 obligations**

---

## 2. 版本与发布模式

| | 本项目 | new-minio |
|--|--------|-----------|
| 基线版本 | `RELEASE.2021-04-22T15-44-28Z` | 上游最新社区版（2026-02 仍在更新 README） |
| Go 版本 | Go 1.16 | Go 1.24 |
| 二进制发布 | 有预编译包 | **仅源码分发**，不再维护社区版预编译二进制 |
| 维护状态 | 你们 fork 的固定快照 | 官方 README 标注 **不再维护**，推荐 AIStor Free/Enterprise |

---

## 3. 架构重构

**代码组织大改：**

- 本项目：`pkg/` 公开包 + `cmd/` 业务逻辑
- new-minio：`pkg/` → `internal/`（不对外暴露），公共库抽到 `github.com/minio/pkg/v3`

**新增核心基础设施：**

- **`internal/grid`**：节点间双向通信框架，替代部分 REST 调用，用于分布式协调
- **`erasure-server-pool`** 大幅增强：支持 pool 下线（decommission）、rebalance、多 pool 管理
- 配置、加密、日志、HTTP 等从 `cmd/config`、`cmd/crypto` 等迁入 `internal/`

---

## 4. 被移除的功能（本项目有，new-minio 无）

| 移除项 | 说明 |
|--------|------|
| **Gateway 模式** | Azure/GCS/HDFS/NAS/S3 网关全部删除；`main.go` 不再 `_ "gateway"` 导入 |
| **嵌入式 Web UI (browser/)** | 整个 `browser/` 前端目录移除 |
| **FS 单机模式 (`fs-v1`)** | 仅支持 Erasure 分布式部署 |
| **Disk Cache** | 磁盘缓存层整套移除 |
| **mint 测试套件** | 兼容性测试框架不再随仓库分发 |
| **web-handlers** | 内置 Web 管理接口移除 |

---

## 5. 新增的核心能力（new-minio 有，本项目无）

约 **188 个** `cmd/` 级新文件，主要包括：

| 新功能 | 代表文件 |
|--------|----------|
| **站点复制 (Site Replication)** | `site-replication.go`, `admin-handlers-site-replication.go` |
| **Batch 批处理作业** | `batch-expire/replicate/rotate.go` |
| **存储分层 / 温存 (Tiering)** | `tier.go`, `warm-backend-*.go` |
| **Pool 下线与再均衡** | `erasure-server-pool-decom.go`, `rebalance-admin.go` |
| **Object Lambda** | `object-lambda-handlers.go` |
| **SFTP/FTP 服务** | `sftp-server.go`, `ftp-server.go` |
| **身份联邦 (IDP)** | `admin-handlers-idp-ldap/openid.go` |
| **Metrics v3** | `metrics-v3-*.go` 全套 Prometheus 指标 |
| **数据扫描器增强** | `data-scanner-metric.go` |
| **Helm 部署** | `helm/`, `helm-releases/` 内置 Chart |
| **Speedtest / ILM** | `speedtest.go`, `ilm-config.go` |

---

## 6. 对你项目的实际影响

```
本项目 (Apache 2.0, 2021)          new-minio (AGPL v3, 2026)
├── 可闭源集成 ✅                   ├── 必须开源或买商业许可 ⚠️
├── Gateway / FS / Browser ✅       ├── 仅 Erasure 分布式 ❌
├── Go 1.16                         ├── Go 1.24
└── 功能较旧但合规简单               └── 功能强但 AGPL 约束严
```

**结论：**

1. **不能直接把 new-minio 当 drop-in 替换**——协议从 Apache 2.0 变为 AGPL v3，闭源产品无法直接使用。
2. **功能差距很大**——分布式能力（Grid、Site Replication、Tiering、Pool 管理）是 5 年演进的结果，不是小 patch。
3. **部署模式也变了**——Gateway、单机 FS、内置 Browser 都没了，只剩 Erasure 集群模式。
4. 若需要新功能又不想开源，官方路线是 **AIStor**（商业许可）；若可接受 AGPL，需自行 `go install` / `go build` 从源码构建。

需要的话，我可以再按模块（存储引擎、IAM、复制、监控等）做更细的代码级 diff 分析。