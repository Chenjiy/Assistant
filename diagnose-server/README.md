# Diagnose Server

## 项目简介
- 一个基于 Gin 的简易诊断服务，提供试卷诊断与练习生成。
- 诊断结果与练习数据可由内置兜底逻辑生成，也可调用兼容 OpenAI 的内部 AI 服务生成严格 JSON。

## 快速开始
- 依赖：Go 1.22+（或兼容版本）
- 安装依赖：无外部依赖，直接构建运行
- 启动服务：
  - `GOROOT=/opt/homebrew/opt/go/libexec go run .`（macOS Homebrew 示例）
  - 或 `go run .`
- 监听端口：`:3000`

## 路由
- `GET /omg/getDiagnoseList`
- `GET /omg/getExercise`

> 注意：虽然是 GET 路由，服务端处理函数会从请求体读取 JSON；因此 curl 需要通过 `-d` 传 JSON，并带上 `Content-Type: application/json`。

## 接口说明

### 1) 试卷诊断：GET /omg/getDiagnoseList
- 请求体：
```json
{
  "ImgLink": [
    "https://example.com/paper-1.jpg",
    "https://example.com/paper-2.jpg"
  ]
}
```
- curl 示例：
```
curl -X GET http://localhost:3000/omg/getDiagnoseList \
  -H 'Content-Type: application/json' \
  -d '{"ImgLink":["https://example.com/paper-1.jpg","https://example.com/paper-2.jpg"]}'
```
- 响应示例（兜底）：
```json
{
  "report": {
    "Conclusion": "总体：基础较好，重点突破计算与逻辑",
    "KSMAnalysis": [
      {"Title": "知识点", "Description": "解析几何与导数为短板"},
      {"Title": "技能", "Description": "计算准确性需提升"},
      {"Title": "心态", "Description": "遇到繁琐运算易焦虑"}
    ],
    "StudyMethod": [
      {"Title": "模板化", "Description": "导数分类讨论流程化"},
      {"Title": "限时训练", "Description": "解析几何耐力题日练"}
    ]
  },
  "ScoreSpace": 30,
  "diagnoseList": [
    {"Title": "全等三角形判定概念", "Degree": "70%", "Status": 0, "ExpectScore": 8, "Description": "分析：基础扎实", "isDiagnose": true},
    {"Title": "解析几何综合", "Degree": "55%", "Status": 1, "ExpectScore": 10, "Description": "分析：运算耐力不足", "isDiagnose": true},
    {"Title": "导数与函数性质", "Degree": "40%", "Status": 2, "ExpectScore": 12, "Description": "分析：分类讨论不完整", "isDiagnose": true}
  ]
}
```
- 代码参考：`diagnose-server/diagnose/getDiagnoseList.go:14-28`、`diagnose-server/diagnose/getDiagnoseList.go:30-84`

### 2) 练习生成：GET /omg/getExercise
- 请求体：
```json
{
  "Title": "三角形全等判定",
  "Description": "考查SSS/SAS/ASA概念及纠错"
}
```
- curl 示例：
```
curl -X GET http://localhost:3000/omg/getExercise \
  -H 'Content-Type: application/json' \
  -d '{"Title":"三角形全等判定","Description":"考查SSS/SAS/ASA概念及纠错"}'
```
- 响应示例（兜底）：
```json
{
  "Title": "三角形全等判定",
  "Concepts": "概念：三角形全等判定",
  "WarnInfo": "注意：仔细审题，流程化表达",
  "Questions": [
    {"Title": "下列判定中正确的是？", "Select": ["SSS","SAS","ASA","AAA"], "CorrectAnswer": "SAS"},
    {"Title": "两边及夹角相等可判定全等吗？", "Select": ["可以","不可以","取决于角度","无法判断"], "CorrectAnswer": "可以"},
    {"Title": "关于解析几何的说法正确的是？", "Select": ["判别式只用于二次方程","韦达定理可用于系数关系","抛物线无焦点","椭圆离心率恒为1"], "CorrectAnswer": "韦达定理可用于系数关系"}
  ]
}
```
- 代码参考：`diagnose-server/diagnose/getDiagnoseExercise.go:11-27`、`diagnose-server/diagnose/getDiagnoseExercise.go:29-66`

## AI 服务
- 采用兼容 OpenAI 的 Chat Completions 接口：`http://ai-service.tal.com/openai-compatible/v1/chat/completions`
- 请求头包含鉴权：`Authorization: Bearer <appID>:<appKey>`
- 当前凭证写在代码里，生产环境建议改为使用环境变量或配置文件加载。
- JSON 抽取工具：`diagnose-server/utils/util.go:5-21`

## 项目结构
```
diagnose-server/
├─ main.go
├─ route/
│  └─ route.go
├─ diagnose/
│  ├─ getDiagnoseList.go
│  └─ getDiagnoseExercise.go
├─ model/
│  └─ diagnose.go
└─ utils/
   └─ util.go
```

## 开发与调试
- 构建：`go build ./...`
- 运行：`go run .`
- 日志：Gin 默认 debug 模式，生产建议设置 `GIN_MODE=release`
- 本地验证：参考上文 curl 示例

## 行为说明
- AI 调用失败或解析异常时，自动回退到兜底数据以保证接口可用。
- 两个接口均严格要求返回结构体对应的 JSON 字段，便于前端稳定解析。

## License
- 内部示例项目，按需调整。
