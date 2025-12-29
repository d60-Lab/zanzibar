# 配置文件说明

本目录包含项目的配置文件。

## 文件列表

### config.example.yaml

配置模板文件，包含所有配置项的示例和说明。此文件会提交到代码仓库中。

**首次使用时**，请复制此文件为 `config.yaml` 并根据实际情况修改配置：

```bash
cp config/config.example.yaml config/config.yaml
```

### config.yaml

本地开发环境配置文件。此文件包含本地环境的实际配置，**不会提交到代码仓库**（已在 `.gitignore` 中排除）。

### config.docker.yaml

Docker 环境配置文件。用于 Docker/Docker Compose 部署时使用。此文件会提交到代码仓库中。

主要差异：

- 数据库 host 使用 Docker 容器服务名（如 `postgres`）而不是 `localhost`
- Redis host 使用 Docker 容器服务名（如 `redis`）而不是 `localhost`
- 服务器模式默认为 `release`
- Sentry 采样率调整为生产环境推荐值

## 配置加载优先级

应用程序会按以下顺序查找配置文件：

1. `./config/config.yaml` - 优先使用当前目录下的配置文件
2. `./config.yaml` - 如果上述文件不存在，使用根目录下的配置文件

## 环境变量覆盖

配置支持通过环境变量覆盖，这在 Docker 部署或生产环境中特别有用。

环境变量命名规则：使用下划线分隔的大写字母，例如：

```bash
# 覆盖数据库密码
export DATABASE_PASSWORD=your_secure_password

# 覆盖 JWT secret
export JWT_SECRET=your_jwt_secret

# 覆盖 Redis 密码
export REDIS_PASSWORD=your_redis_password
```

## 配置项说明

### Server 配置

- `port`: 服务监听端口
- `mode`: 运行模式 (debug, release, test)
- `read_timeout`: 读取超时时间（秒）
- `write_timeout`: 写入超时时间（秒）

### Database 配置

- `driver`: 数据库驱动（目前支持 postgres）
- `host`: 数据库主机地址
- `port`: 数据库端口
- `database`: 数据库名称
- `username`: 数据库用户名
- `password`: 数据库密码（**生产环境建议使用环境变量**）
- `max_open_conns`: 最大打开连接数
- `max_idle_conns`: 最大空闲连接数
- `conn_max_lifetime`: 连接最大生命周期（秒）

### Redis 配置

- `host`: Redis 主机地址
- `port`: Redis 端口
- `password`: Redis 密码（**生产环境建议使用环境变量**）
- `db`: Redis 数据库编号

### JWT 配置

- `secret`: JWT 签名密钥（**生产环境必须修改并使用环境变量**）
- `expire`: Token 过期时间（秒）

### Pprof 配置

- `enabled`: 是否启用性能分析（生产环境建议关闭）

### Sentry 配置

- `enabled`: 是否启用 Sentry 错误追踪
- `dsn`: Sentry DSN
- `environment`: 环境名称（development, staging, production）
- `traces_sample_rate`: 追踪采样率（0.0-1.0）
- `debug`: 是否启用调试模式

### Tracing 配置

- `enabled`: 是否启用 OpenTelemetry 追踪
- `service_name`: 服务名称
- `jaeger_endpoint`: Jaeger 收集器端点

## 安全建议

1. **永远不要**将包含敏感信息的 `config.yaml` 提交到代码仓库
2. 生产环境的敏感配置（如密码、密钥）应使用环境变量
3. 定期更换 JWT secret 等安全密钥
4. 为不同环境使用不同的配置值
5. 使用强密码和复杂的密钥
