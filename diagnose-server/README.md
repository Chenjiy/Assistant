# OMG-AI 试卷精算 - 诊断服务 (Diagnose Server)

这是一个基于 Go 语言开发的后端诊断服务，用于处理试卷相关的诊断和练习数据。

## 🚀 项目介绍

该项目是 OMG-AI 试卷精算系统的核心组件之一，主要负责：
- 获取诊断列表
- 获取特定诊断的练习题目
- 路由管理及接口对接

## 🛠️ 技术栈

- **语言**: Go (推荐版本 1.23+)
- **框架**: [Gin Web Framework](https://github.com/gin-gonic/gin)

## 📋 环境要求

- **Go**: >= 1.23.0
- **网络**: 能够访问 github.com 以拉取依赖

## 🏃 运行指南

请确保你已进入项目目录 `Assistant/diagnose-server`。

### 1. 安装依赖

```bash
cd Assistant/diagnose-server
go mod download
```

### 2. 运行服务

```bash
go run main.go
```

服务默认运行在 [http://localhost:3000](http://localhost:3000)

## 📂 项目结构

```text
diagnose-server/
├── main.go           # 程序入口
├── route/           # 路由定义
├── diagnose/        # 诊断逻辑处理 (业务层)
├── model/           # 数据模型定义
├── utils/           # 工具函数
└── go.mod           # 依赖管理
```

