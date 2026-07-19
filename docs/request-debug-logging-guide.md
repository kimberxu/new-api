# 请求调试日志操作指引

本文档说明如何启用、验证、使用和维护本地定制的请求调试日志功能。该功能用于比较客户端提交的下游 JSON 请求与系统完成模型映射、字段删除和参数覆盖后实际发送给渠道的上游 JSON 请求。

## 1. 功能用途

当请求因为以下原因失败或行为异常时，可使用请求调试快照定位差异：

- 模型映射与预期不一致；
- 渠道禁用字段被删除；
- 渠道参数覆盖修改了请求；
- OpenAI、Claude、Gemini 或 Responses 格式转换异常；
- 透传模式下客户端请求与渠道响应不符合预期；
- 同一请求经过重试后，不同渠道收到的参数不同。

快照存放在既有日志的 `Other.admin_info.request_debug` 中，不新增数据库表或字段。普通用户查询日志时，后端会删除整个 `admin_info`，因此只有管理员能够读取快照。

## 2. 安全警告

请求调试快照可能包含提示词、消息内容、工具参数、文件地址以及其他业务数据。启用前必须理解以下限制：

1. 功能默认关闭，不建议长期使用 `always`。
2. 系统只会自动脱敏已识别的凭据字段：

   ```text
   authorization
   api_key
   apikey
   access_token
   refresh_token
   key
   token
   password
   secret
   ```

3. 字段名称匹配不区分大小写，并会忽略字段名前后的空格。
4. 提示词、聊天内容及名称不在上述列表中的自定义敏感字段不会自动脱敏。
5. 不要把管理员日志导出到公开工单、聊天群或不受信任的日志平台。
6. 管理员账号应启用强密码和最小权限控制。
7. 排障结束后应立即恢复为 `off`。

生产环境推荐只临时启用 `error_only`。

## 3. 配置项

### 3.1 `REQUEST_DEBUG_LOGGING`

控制快照的记录模式：

| 值 | 成功日志 | 错误日志 | 建议用途 |
| --- | --- | --- | --- |
| `off` | 不记录 | 不记录 | 默认值、正常生产运行 |
| `error_only` | 不记录 | 记录 | 生产环境临时排障 |
| `always` | 记录 | 记录 | 本机或隔离测试环境验证 |

值会自动转换为小写并清理首尾空格。空值或其他无效值都会按 `off` 处理。

### 3.2 `REQUEST_DEBUG_MAX_BODY_BYTES`

控制每一侧请求体最多保存多少字节：

```env
REQUEST_DEBUG_MAX_BODY_BYTES=32768
```

默认值为 `32768`，即 32 KiB。设置为 `0`、负数或无法解析的值时会恢复默认值。

超过限制的请求体会被截断，但日志仍会保存：

- 原始请求体字节数 `size`；
- 完整原始请求体的 SHA-256 `sha256`；
- 截断状态 `truncated`；
- 脱敏和截断后的内容 `body`。

大字符串值会先单独截断，再计算最终保存内容，避免将完整 Base64、Data URL 或其他超长字符串写入日志。

## 4. 推荐配置

### 4.1 默认生产配置

```env
REQUEST_DEBUG_LOGGING=off
REQUEST_DEBUG_MAX_BODY_BYTES=32768
```

### 4.2 临时排查失败请求

```env
REQUEST_DEBUG_LOGGING=error_only
REQUEST_DEBUG_MAX_BODY_BYTES=32768
```

### 4.3 本机功能验证

```env
REQUEST_DEBUG_LOGGING=always
REQUEST_DEBUG_MAX_BODY_BYTES=4096
```

本机验证时使用较小上限，可更容易确认截断逻辑。

## 5. 部署步骤

### 5.1 先确认当前部署方式

如果 Compose 中使用的是：

```yaml
services:
  new-api:
    image: calciumion/new-api:latest
```

那么容器运行的是上游公开镜像。即使服务器上的 Git 仓库已经切换到 `deploy` 分支，执行普通的 `docker compose up -d` 仍不会包含本地请求调试改造。

本地定制版本必须通过本仓库的 `Dockerfile` 构建。推荐在部署机器上从个人 Gitea 的 `deploy` 分支构建，并给本地镜像使用独立名称，避免覆盖上游镜像。

本地 Gitea 仓库使用以下分支：

- `local/request-debug`：请求调试功能分支；
- `deploy`：部署分支；
- `main`：个人主分支。

完整 Git 工作流见 [local-gitea-workflow.md](local-gitea-workflow.md)。

### 5.2 合并并更新 Gitea 部署分支

在开发机完成验证后执行：

```bash
git checkout deploy
git pull --ff-only origin deploy
git merge --no-ff local/request-debug
git push origin deploy
```

### 5.3 首次将原上游克隆切换到个人 Gitea

以下命令只需执行一次。先进入原来运行 Compose 的仓库目录：

```bash
cd /实际路径/new-api
git remote -v
```

如果当前只有名为 `origin` 的 GitHub 上游地址，将它保留为 `upstream`，再添加个人 Gitea 为新的 `origin`：

```bash
git remote rename origin upstream
git remote add origin ssh://git@gitea.228778.xyz:23022/kim/new-api.git
git fetch origin
git fetch upstream
```

如果仓库已经存在 `upstream`，不要再次执行 `git remote rename`。应检查地址并按实际情况设置：

```bash
git remote set-url upstream https://github.com/QuantumNous/new-api.git
git remote set-url origin ssh://git@gitea.228778.xyz:23022/kim/new-api.git
git fetch --all --prune
```

不要在未检查 `git remote -v` 的情况下盲目覆盖远程地址。

### 5.4 备份现有部署配置

现有 `.env`、Compose 文件、数据库目录和日志目录都应保留。切换分支前先备份：

```bash
cd /实际路径/new-api
cp .env .env.before-request-debug
cp docker-compose.yml docker-compose.yml.before-request-debug
git status --short
```

如果实际文件名是 `docker-compose.yaml` 或 `compose.yml`，命令中的文件名应同步替换。

记录当前上游镜像 ID，并额外创建一个本地回滚标签：

```bash
docker image inspect calciumion/new-api:latest --format '{{.Id}}'
docker tag calciumion/new-api:latest new-api-upstream-backup:pre-request-debug
```

`./data` 和 `./logs` 是绑定挂载目录，不要删除、移动或执行 `docker compose down -v`。

### 5.5 切换到 `deploy` 分支

```bash
cd /实际路径/new-api
git fetch origin
git checkout -B deploy origin/deploy
git pull --ff-only origin deploy
```

如果 `git checkout` 提示本地修改会被覆盖，停止操作。先确认修改是否只来自 `.env` 和 Compose 文件；不要使用 `git reset --hard`。使用前一步的备份恢复部署配置，或把机器专用 Compose 文件改成不受 Git 跟踪的名称。

### 5.6 创建机器专用 Compose 文件

推荐保留仓库的 `docker-compose.yml`，把原生产配置复制成机器专用文件：

```bash
cp docker-compose.yml.before-request-debug docker-compose.production.yml
```

编辑 `docker-compose.production.yml`，仅将 `new-api` 服务的公开镜像配置：

```yaml
image: calciumion/new-api:latest
```

替换为：

```yaml
build:
  context: .
  dockerfile: Dockerfile
image: new-api-local:request-debug
```

根据用户现有配置，`new-api` 服务应类似：

```yaml
services:
  new-api:
    mem_limit: 2048m
    build:
      context: .
      dockerfile: Dockerfile
    image: new-api-local:request-debug
    container_name: new-api
    restart: unless-stopped
    command: --log-dir /app/logs
    ports:
      - "23002:3000"
    volumes:
      - ./data:/data
      - ./logs:/app/logs
    env_file:
      - .env
    depends_on:
      - redis
```

原有 Redis、数据库、healthcheck、端口、数据卷和其他环境配置保持不变。不要把现有 `.env` 内容复制到镜像或提交到 Git。

先检查 Compose 语法和最终合并结果：

```bash
docker compose -f docker-compose.production.yml config
```

如果机器使用旧版命令，将 `docker compose` 替换为 `docker-compose`。

### 5.7 修改 `.env`

### 5.3 配置文件部署

如果使用 `.env`，加入：

```env
REQUEST_DEBUG_LOGGING=off
REQUEST_DEBUG_MAX_BODY_BYTES=32768
```

现有 Compose 已通过：

```yaml
env_file:
  - .env
```

读取同目录的 `.env`，因此不需要再把相同变量重复写入 `environment`。

首次上线保持 `REQUEST_DEBUG_LOGGING=off`。修改 `.env` 后必须重建或重新创建 `new-api` 容器，运行中的容器不会自动读取新值。

### 5.8 构建本地镜像

仓库 `Dockerfile` 会依次构建 default 前端、classic 前端和 Go 后端，因此首次构建需要访问 Bun、Go 和 Debian 软件源，并会占用一定时间和磁盘空间。

执行：

```bash
docker compose -f docker-compose.production.yml build --pull new-api
```

构建完成后确认本地镜像存在：

```bash
docker image inspect new-api-local:request-debug --format '{{.Id}} {{.Created}}'
```

构建失败时不要停止现有容器；修复网络、磁盘或依赖问题后重新构建即可。

### 5.9 替换后端容器

构建成功后只更新 `new-api` 服务：

```bash
docker compose -f docker-compose.production.yml up -d --no-deps new-api
docker compose -f docker-compose.production.yml ps
docker logs --tail 100 new-api
```

`--no-deps` 可避免无必要地重建或重启 Redis。由于继续使用相同的容器名、端口和绑定挂载目录，原有 `./data`、`./logs` 与外部数据库配置会继续使用。

检查健康状态：

```bash
docker inspect new-api --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}'
curl -fsS http://127.0.0.1:23002/api/status
```

### 5.10 后续更新

以后更新个人部署分支时执行：

```bash
cd /实际路径/new-api
git fetch origin
git checkout deploy
git pull --ff-only origin deploy
docker compose -f docker-compose.production.yml build new-api
docker compose -f docker-compose.production.yml up -d --no-deps new-api
docker compose -f docker-compose.production.yml ps
```

不要再执行只会拉取上游公开镜像的：

```bash
docker compose pull new-api
```

除非你的目标是明确回退到 `calciumion/new-api`。

## 6. 快照结构

管理员日志中的典型结构如下：

```json
{
  "admin_info": {
    "request_debug": {
      "mode": "error_only",
      "request_path": "/v1/chat/completions",
      "relay_mode": 1,
      "content_type": "application/json",
      "downstream": {
        "size": 256,
        "sha256": "完整原始请求体的 SHA-256",
        "truncated": false,
        "body": "{\"model\":\"gpt-example\",\"api_key\":\"[REDACTED]\"}"
      },
      "upstream": {
        "size": 241,
        "sha256": "完整上游请求体的 SHA-256",
        "truncated": false,
        "body": "{\"model\":\"mapped-model\",\"api_key\":\"[REDACTED]\"}"
      }
    }
  }
}
```

字段含义：

| 字段 | 含义 |
| --- | --- |
| `mode` | 捕获快照时使用的配置模式 |
| `request_path` | 请求路径，不包含查询参数 |
| `relay_mode` | 后端内部 relay 模式编号 |
| `content_type` | 客户端请求的 `Content-Type` |
| `downstream` | 客户端发送给本系统的请求体 |
| `upstream` | 本系统最终发送给渠道的请求体 |
| `size` | 脱敏和截断前的原始字节数 |
| `sha256` | 完整原始请求体摘要，可用于判断两次请求是否一致 |
| `truncated` | 保存内容是否因大小限制被截断 |
| `body` | 脱敏、字符串限制和整体截断后的内容 |
| `request_debug_error` | 快照采集失败原因；该错误不会中断正常 relay |

## 7. 排障方法

### 7.1 参数覆盖问题

比较 `downstream.body` 和 `upstream.body`：

- 检查模型名称是否发生映射；
- 检查参数覆盖规则是否新增、替换或删除字段；
- 检查数组、工具定义或系统提示词是否发生变化。

上游快照的采集点位于最终 JSON 变换完成之后、创建出站请求体之前，因此应与实际发送内容一致。

### 7.2 禁用字段问题

如果某字段存在于 `downstream.body`，但不存在于 `upstream.body`：

1. 检查渠道的禁用字段设置；
2. 检查是否启用了请求体透传；
3. 检查对应协议转换是否支持该字段；
4. 检查参数覆盖规则是否执行了删除操作。

### 7.3 模型映射问题

确认：

- `downstream.body` 中是客户端请求模型；
- `upstream.body` 中是渠道实际模型；
- 日志的模型映射信息与 `upstream.body` 一致。

### 7.4 快照被截断

当 `truncated` 为 `true` 时：

1. 优先根据 `sha256` 判断请求是否相同；
2. 不要直接在生产环境无限提高存储上限；
3. 如确需查看更多内容，可短时间提高 `REQUEST_DEBUG_MAX_BODY_BYTES`；
4. 排障后恢复为 `32768` 或更低值。

### 7.5 只有下游快照，没有上游快照

常见原因：

- 请求在格式转换之前失败；
- 模型映射、请求校验或参数覆盖报错；
- 当前请求不是首版支持的 JSON relay 路径；
- 请求体存储读取失败，此时检查 `request_debug_error`。

### 7.6 日志中完全没有 `request_debug`

依次检查：

1. `REQUEST_DEBUG_LOGGING` 是否为 `always` 或 `error_only`；
2. 修改环境变量后是否重启服务；
3. `error_only` 模式下当前请求是否确实记录了错误日志；
4. 当前账号是否为管理员；
5. 请求是否走受支持的 OpenAI Compatible、Responses、Claude 或 Gemini JSON 路径。

## 8. 无独立测试实例时的上线策略

没有独立测试实例时，采用以下低风险流程：

1. 合并并部署代码，但保持：

   ```env
   REQUEST_DEBUG_LOGGING=off
   ```

2. 使用 `docker compose -f docker-compose.production.yml ps` 和 `/api/status` 确认容器健康。
3. 确认实际运行的是本地镜像：

   ```bash
   docker inspect new-api --format '{{.Config.Image}}'
   ```

   预期输出：

   ```text
   new-api-local:request-debug
   ```

4. 确认服务正常启动，普通请求行为不变。
5. 仅在需要排查真实失败时切换为：

   ```env
   REQUEST_DEBUG_LOGGING=error_only
   ```

6. 使用以下命令重新创建后端容器，使 `.env` 生效：

   ```bash
   docker compose -f docker-compose.production.yml up -d --no-deps --force-recreate new-api
   ```

7. 观察管理员错误日志。
8. 收集到足够信息后立即恢复 `off`，并再次重新创建后端容器。

不要为了验证功能而在生产渠道上故意配置错误密钥，也不要制造可能触发渠道自动禁用的失败请求。

## 9. 自动化验证

聚焦测试：

```bash
go test ./common -run InitRequestDebugConfig -count=1
go test ./relay/common -run RequestDebug -count=1
go test ./service -run RequestDebug -count=1
go test ./model -run FormatUserLogs -count=1
```

相关包验证：

```bash
go test ./common ./controller ./model ./relay ./relay/common ./service \
  -run 'RequestDebug|FormatUserLogs|InitRequestDebugConfig' -count=1
```

完整测试：

```bash
go test ./...
```

当前仓库执行完整测试时可能受以下既有环境问题影响：

- 根包需要存在 `web/classic/dist` 才能满足 `go:embed`；
- 部分 service 指标测试共同运行时可能因进程级计数器状态互相影响，但单独运行可通过。

不能因为这些已知问题忽略请求调试相关聚焦测试的失败。

## 10. 关闭与回滚

### 10.1 立即关闭采集

设置：

```env
REQUEST_DEBUG_LOGGING=off
```

然后重启服务。关闭后不会生成新的请求调试快照，已有数据库日志不会自动删除。

### 10.2 清理已有日志

请求快照保存在既有日志 `Other` 字段中。不要直接编写跨数据库不兼容的 SQL 批量修改生产日志。

如果必须删除已有快照，应先：

1. 明确日志时间范围和日志 ID；
2. 备份数据库；
3. 根据实际数据库类型设计兼容方案；
4. 在副本上验证；
5. 再执行生产清理。

仅关闭环境变量不会删除历史数据。

### 10.3 代码回滚

优先回滚到已知正常部署标签：

```bash
git checkout deploy
git tag deploy-YYYYMMDD-before-request-debug
git push origin deploy-YYYYMMDD-before-request-debug
```

如果部署后发现异常，先将 `REQUEST_DEBUG_LOGGING` 设置为 `off` 并重新创建容器。只有确认问题由代码本身引起时，再回退到之前保留的上游镜像。

将 `docker-compose.production.yml` 中：

```yaml
build:
  context: .
  dockerfile: Dockerfile
image: new-api-local:request-debug
```

替换为：

```yaml
image: new-api-upstream-backup:pre-request-debug
```

然后执行：

```bash
docker compose -f docker-compose.production.yml config
docker compose -f docker-compose.production.yml up -d --no-deps new-api
docker compose -f docker-compose.production.yml ps
docker logs --tail 100 new-api
```

该回滚不会删除 `./data` 或 `./logs`。不要执行 `docker compose down -v`。

如未创建本地回滚标签，可临时恢复为：

```yaml
image: calciumion/new-api:latest
```

但 `latest` 可能已经指向不同版本，因此可靠性低于部署前保存的固定镜像。

## 11. 上游同步与本地维护

该功能是本地定制，应保持为单个聚焦提交，便于在同步上游后重新应用。

更新上游后：

```bash
git fetch upstream
git checkout main
git pull --ff-only origin main
git merge upstream/main
git push origin main
```

如果请求调试改造冲突较多：

```bash
git checkout main
git checkout -b local/request-debug-refresh
git cherry-pick d25a0524
```

解决冲突后重新运行第 9 节的聚焦测试，再合并到 `deploy`。

预期容易发生冲突的文件包括：

- `common/constants.go`
- `common/init.go`
- `controller/relay.go`
- `relay/common/relay_info.go`
- `relay/common/request_debug.go`
- `relay/compatible_handler.go`
- `relay/responses_handler.go`
- `relay/claude_handler.go`
- `relay/gemini_handler.go`
- `service/log_info_generate.go`

不要把快照逻辑复制到各个渠道 adaptor 中；应继续集中维护脱敏、截断和快照组装逻辑。

## 12. 日常操作速查

启用失败请求采集：

```env
REQUEST_DEBUG_LOGGING=error_only
REQUEST_DEBUG_MAX_BODY_BYTES=32768
```

排障完成后关闭：

```env
REQUEST_DEBUG_LOGGING=off
```

检查配置后必须重启服务。管理员查看 `Other.admin_info.request_debug`，普通用户不应看到 `admin_info`。
