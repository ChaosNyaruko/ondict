# Sync server 部署与现有数据迁移指南

本文面向已经在用 ondict（以 `wordbank.db` / `history.db` 形式持久化）并希望开启**云端同步**（ADR 0001）的运维者。
设计依据见 [`adr/0001-wordbank-history-sync.md`](adr/0001-wordbank-history-sync.md)。

## 目录

- [1. 角色与目录约定](#1-角色与目录约定)
- [2. Schema 自动迁移说明（v0 → v3）](#2-schema-自动迁移说明v0--v3)
- [3. 启用 sync server 的最小步骤](#3-启用-sync-server-的最小步骤)
- [4. 把现有 `~/.config/ondict/{wordbank,history}.db` 转成 sync 用户库](#4-把现有-configondictwordbankhistorydb-转成-sync-用户库)
- [5. 客户端接入](#5-客户端接入)
- [6. 验证清单](#6-验证清单)
- [7. 常见问题](#7-常见问题)

---

## 1. 角色与目录约定

ondict 在同一台机器上有两种"身份"，对应两套独立的 SQLite 文件：

| 身份 | 数据库路径 | 用途 |
|---|---|---|
| **本地 ondict 客户端** | `~/.config/ondict/{wordbank,history}.db` | 这台机器自己 `ondict -q` / 浏览器查询时的本地状态 |
| **sync server**（启用 `-sync-server` 时） | `<sync-data-dir>/<user>/{wordbank,history}.db` | 多设备汇聚的"用户云端库"，每用户一份 |

两者**互不感知**：sync server 不会自动读你的本地 `~/.config/ondict/wordbank.db`，本地 ondict 客户端也不会自动把数据 push 到 sync server。要打通的话需要：

- 让本地的 ondict 也跑 `ondict sync --base-url ...` 去推（参见 §5）
- 或者把现有库**离线复制 / 合并**到 sync server 的用户目录（参见 §4）

`<sync-data-dir>` 由 `-sync-data-dir` 指定；不传则默认 `<config>/sync`。

## 2. Schema 自动迁移说明（v0 → v3）

每次 ondict 打开任何一个 wordbank/history DB，都会自动执行 `dbutil.EnsureSchema`，按 `meta` 表里的 `schema_version` 决定要不要继续推进。三个版本依次解锁的能力：

| 版本 | 关键变更 | 解锁能力 |
|---|---|---|
| v1 | 建 `meta` 表登记 schema_version；按 v0 legacy 形状 `CREATE TABLE IF NOT EXISTS` 守护已有库 | 把现状登记进 migration 框架 |
| v2 | 两个 DB 加 `deleted_at` 墓碑列；`history` 旧 localtime 行按本机 `time.Local` 重新解释成 UTC；补 `i_count`/`i_latest`/`i_words_update_time` 索引 | 软删可同步（D6）+ 时间统一 UTC（merge 引擎可用） |
| v3 | 两个 DB 加 `server_seen_at` 列（nullable + 同事务回填）；加 `i_*_server_seen_at` 索引；runtime 写入切到毫秒精度 `strftime('%f','now')` | sync delta 过滤可用（D11） |

迁移特性：
- **幂等**：已经到 v3 的库再次启动只会读一下 `meta`，不做任何修改
- **forward-only**：没有反向迁移；务必先备份（参见 §3 步骤 1）
- **事务保护**：每条 migration 在事务里跑，失败回滚到上一个 version，不会留中间态

> ⚠️ history v1→v2 的时区猜测：旧 `history.db` 里的 `update_time` 是当年写入机器的本地时区，但没存时区元信息。迁移时只能按**当前机器**的 `time.Local` 反解。如果你以前在不同时区的机器之间搬过 `history.db`，旧行的时间戳会偏移几小时——影响 `Review` 的边界，merge 不影响。

## 3. 启用 sync server 的最小步骤

### 3.1 备份（强烈建议）

```bash
cp ~/.config/ondict/wordbank.db ~/.config/ondict/wordbank.db.v0.bak
cp ~/.config/ondict/history.db  ~/.config/ondict/history.db.v0.bak
```

> 迁移是单向的，备份是回退的唯一手段。

### 3.2 替换二进制

```bash
go install github.com/ChaosNyaruko/ondict@latest
# 或本地编译：make install
```

第一次访问任意 wordbank/history DB 时，schema 会自动迁移到 v3。

### 3.3 设置凭据并启动 sync server

```bash
# 把凭据放在 env 里，避免进 shell history
export ONDICT_SYNC_USER=alice
export ONDICT_SYNC_PASSWORD='something-strong-with-32+-bytes'

# 启动；TLS 由前置的 Caddy / nginx 处理（ADR D10）
ondict -serve \
       -listen=:1345 \
       -sync-server \
       -sync-data-dir=/var/lib/ondict/sync
```

第一次启动时 `/var/lib/ondict/sync/` 是**空目录**——里面的 `<user>/{wordbank,history}.db` 是 client 第一次 push 时才会被创建的。如果想让 alice 上线就有数据，看 §4。

### 3.4 反向代理 + TLS（推荐 Caddy）

`Caddyfile`：

```caddy
sync.example.com {
    reverse_proxy localhost:1345
}
```

Caddy 自动申请 / 续期 Let's Encrypt 证书，转发到本地 ondict。

### 3.5 验证

```bash
# 401，因为没带凭据
curl -i https://sync.example.com/sync/v1/state

# 200 + JSON
curl -i -u "alice:something-strong-with-32+-bytes" \
     https://sync.example.com/sync/v1/state
# {"server_now":"2026-06-04T...","schema_version":1}
```

`schema_version` 是 wire-protocol 版本（见 ADR），和 SQLite schema_version 是两回事。

## 4. 把现有 `~/.config/ondict/{wordbank,history}.db` 转成 sync 用户库

很多人的 server 同时也是这台机器**自己**的 ondict 客户端，已经攒了一段时间的 wordbank / history。新启动 `-sync-server` 后，sync 用户库 `<sync-data-dir>/alice/` 默认是空的，跨设备过来的 alice 看不到这些已有数据。下面给两种方式把已有数据"灌"成 alice 的初始库。

> **务必先关掉正在运行的 ondict 进程**（含 `-serve` 模式），否则 SQLite 会有锁冲突。
>
> ```bash
> systemctl stop ondict   # 或 pkill ondict，看你的部署
> ```

### 4.1 方式 A：直接复制（最快、最简单）

适用场景：sync 用户库目前是空的，或者你确认想用本地库**完全覆盖**它。

```bash
# 1. 创建用户目录
sudo mkdir -p /var/lib/ondict/sync/alice
sudo chown -R $USER /var/lib/ondict/sync/alice  # 让 ondict 进程能写

# 2. 复制
cp ~/.config/ondict/wordbank.db /var/lib/ondict/sync/alice/wordbank.db
cp ~/.config/ondict/history.db  /var/lib/ondict/sync/alice/history.db

# 3. 启动 sync server，会自动跑 EnsureSchema 把库推到 v3（如果还没到）
ondict -serve -listen=:1345 -sync-server -sync-data-dir=/var/lib/ondict/sync
```

**原理**：sync server 第一次访问 `alice/wordbank.db` 时调用 `OpenSQLiteWordbank(path)`，里面会跑 `dbutil.EnsureSchema`，把库推进到当前代码要求的 schema（v3）。已经到 v3 的库会被识别后跳过。

**优缺点**：
- ✅ 简单，一行 cp
- ❌ 覆盖语义：如果 sync 用户库已经有 client 推过的数据，直接 cp 会丢

### 4.2 方式 B：用 `ondict merge` 合并（推荐，幂等可重入）

适用场景：不确定 sync 用户库是不是已经有数据，或者想保留两边的并集。

```bash
# 1. 准备目录
sudo mkdir -p /var/lib/ondict/sync/alice
sudo chown -R $USER /var/lib/ondict/sync/alice

# 2. 先 dry-run 看一眼会发生什么
ondict merge wordbank \
       --dst /var/lib/ondict/sync/alice/wordbank.db \
       --dry-run \
       ~/.config/ondict/wordbank.db

# 输出大致：
# [wordbank] /home/.../wordbank.db: inserted=187 updated=0 unchanged=0 tombstones=0
# [wordbank] TOTAL  -> dst=/var/lib/.../wordbank.db inserted=187 ... (dry-run=true)

# 3. 真正跑
ondict merge wordbank \
       --dst /var/lib/ondict/sync/alice/wordbank.db \
       ~/.config/ondict/wordbank.db

ondict merge history \
       --dst /var/lib/ondict/sync/alice/history.db \
       ~/.config/ondict/history.db

# 4. 启动 server
ondict -serve -listen=:1345 -sync-server -sync-data-dir=/var/lib/ondict/sync
```

**原理**：`ondict merge` 用的是和 sync server / client 同一套 `syncmerge` engine（ADR D2/D3）：
- wordbank：按 `update_time` LWW，`create_time` 取 MIN，墓碑按 LWW 传播
- history：同上 + `count` 取 MAX（D3 幂等）

**优缺点**：
- ✅ 幂等：再跑一次只会增加 `Unchanged` 计数，不会乱写
- ✅ 不会丢 dst 端已有的数据
- ✅ 多个源库可以一次合：`ondict merge wordbank --dst x.db src1.db src2.db src3.db`

### 4.3 方式 C：让本地 ondict 客户端 push 到自己

跳过 §4.1/§4.2 的离线拷贝，让 server 上的本地 ondict 像普通 client 一样推数据：

```bash
# server 上，作为客户端跑一次性 sync
export ONDICT_SYNC_USER=alice
export ONDICT_SYNC_PASSWORD='something-strong-...'
ondict sync --base-url http://127.0.0.1:1345
```

server 端会通过 HTTP push 把本机 `~/.config/ondict/{wordbank,history}.db` 的所有 row 推到 `/var/lib/ondict/sync/alice/`。后续可以加 `--loop 10m` 跑成常驻。

**优缺点**：
- ✅ 不用动文件系统
- ✅ 完全走"正式协议"，不用担心 schema 不一致
- ❌ 第一次推会比较慢（全量），所以中等大小（>10k 行）的库还是建议方式 B

## 5. 客户端接入

### 5.1 桌面（其他机器）

```bash
export ONDICT_SYNC_USER=alice
export ONDICT_SYNC_PASSWORD='something-strong-...'

# 一次性
ondict sync --base-url https://sync.example.com

# 常驻：每 10 分钟同步一次，墓碑保留 90 天
ondict sync --base-url https://sync.example.com \
            --loop 10m \
            --gc-tombstones-after 2160h
```

### 5.2 Android 集成

在 Activity 里：

```kotlin
// 启动 mobile.StartServer(...) 之后
val ok = mobile.Mobile.configureSync(
    "https://sync.example.com",
    "alice",
    BuildConfig.SYNC_PASSWORD
)

// 用户拉刷新或周期触发
val summary = mobile.Mobile.sync()  // 返回 "sync ok: ..."
```

详见 `mobile/mobile.go` 的 `ConfigureSync` / `Sync` 注释。

## 6. 验证清单

依次执行：

```bash
# 1. 确认 schema 已经推进到 v3
sqlite3 ~/.config/ondict/wordbank.db "SELECT * FROM meta;"
# schema_version|3

sqlite3 /var/lib/ondict/sync/alice/wordbank.db "SELECT * FROM meta;"
# schema_version|3

# 2. 看一行示例，确认时间格式是 RFC3339-ms UTC
sqlite3 /var/lib/ondict/sync/alice/wordbank.db \
        "SELECT word, create_time, update_time, server_seen_at FROM words LIMIT 1;"
# alpha|2024-01-01T00:00:00.000Z|2024-01-01T00:00:00.000Z|2026-06-04T07:09:51.234Z

# 3. 确认 /sync/v1/state 有响应
curl -i -u "alice:..." https://sync.example.com/sync/v1/state

# 4. 跑一次客户端 sync，看 stats
ondict sync --base-url https://sync.example.com
# sync ok: wordbank pull(ins=0 upd=0 unc=187) push=0; history pull(...)=0; ...

# 5. 在手机上 ConfigureSync + Sync()，确认它能拉到 §4 灌进去的数据
```

## 7. 常见问题

### Q1：迁移失败怎么回退？

migration 在事务里跑，失败本身会回滚到上一个 schema_version；但如果是想从已经成功的 v3 退回 v0/v1：**没有自动回退**。用 §3.1 的备份覆盖回去：

```bash
systemctl stop ondict
cp ~/.config/ondict/wordbank.db.v0.bak ~/.config/ondict/wordbank.db
cp ~/.config/ondict/history.db.v0.bak  ~/.config/ondict/history.db
# 然后用旧版本二进制启动
```

### Q2：sync 用户库 `<dataDir>/alice/wordbank.db` 已经混乱了想重置？

```bash
systemctl stop ondict
rm -rf /var/lib/ondict/sync/alice
# 启动 server；alice 第一次 push 时会自动建空库
```

如果客户端不希望它的 cursor 记忆失效（避免重新全量 push），那只重置 server 端是不够的——客户端的 `meta` 表里还存着 `sync.last_pull.*` / `sync.last_push.*` 游标。要彻底重置一边的话，把客户端 wordbank.db 的 cursor 行也清掉：

```bash
sqlite3 ~/.config/ondict/wordbank.db \
        "DELETE FROM meta WHERE key LIKE 'sync.%';"
```

### Q3：多用户怎么办？

当前 ADR D4 是**单用户 Basic Auth**。多用户不在 v1 范围；已记录在 ADR "Open follow-ups"。短期 workaround：跑多个 ondict 进程，每个绑不同端口 + 不同 `-sync-data-dir` + 不同凭据 env。

### Q4：sync 服务器和本地客户端能不能用同一个 `wordbank.db`？

**强烈不建议。** 两个 SQLite 进程并发写会有锁竞争和潜在的 schema 一致性问题。正确做法是按 §4 的方式 B 把本地数据 merge 到 sync 用户库，然后让本地 ondict 也通过 `ondict sync --base-url http://127.0.0.1:1345` 同步到这个用户库——本地客户端读写自己的 `~/.config/ondict/wordbank.db`，sync 在后台双向同步。

### Q5：怎么定期 GC 墓碑？

服务器端目前没有自动 GC 任务（D6 follow-up）。最简单方式是让某个 client 跑常驻 sync 时带 `--gc-tombstones-after`：

```bash
ondict sync --base-url ... --loop 10m --gc-tombstones-after 2160h  # 90 天
```

这只清理本地端的墓碑。要清服务器端的墓碑，可以临时在 server 机器上跑一次 `ondict sync --base-url http://127.0.0.1:1345 --gc-tombstones-after 2160h` 让它当成 client 接进来 GC。或者直接 `sqlite3` 手动：

```sql
DELETE FROM words   WHERE deleted_at IS NOT NULL AND deleted_at < '2026-03-01T00:00:00.000Z';
DELETE FROM history WHERE deleted_at IS NOT NULL AND deleted_at < '2026-03-01T00:00:00.000Z';
```

## 参考

- [`adr/0001-wordbank-history-sync.md`](adr/0001-wordbank-history-sync.md) — 完整设计依据，包括 D1–D11 决策和被拒方案
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — 项目级架构概览
- `internal/syncserver/server.go` — server 实现
- `internal/syncclient/client.go` — client 实现
- `syncmerge/syncmerge.go` — 双方共用的 merge 引擎
