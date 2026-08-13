COMPOSE := docker compose -f docker-compose.dev.yml

.PHONY: up build down restart ps logs config infra reset-db

up:
	$(COMPOSE) up -d --build

build:
	$(COMPOSE) build

infra:
	$(COMPOSE) up -d mysql redis etcd etcd-init minio minio-init

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) restart

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f --tail=200

config:
	$(COMPOSE) config --quiet

# 删除开发环境数据卷并重新执行全部数据库初始化脚本。仅可用于本地开发。
reset-db:
	$(COMPOSE) down -v
	$(COMPOSE) up -d --build

