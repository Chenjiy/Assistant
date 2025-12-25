package diagnose

import (
	"bytes"
	"diagnose-server/model"
	"diagnose-server/utils"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func GetDiagnoseExercise(ctx *gin.Context) {
	var req model.GetDiagnoseExerciseRequest
	// 解析参数
	ct := ctx.GetHeader("Content-Type")
	if strings.Contains(ct, "application/json") {
		b, _ := ctx.GetRawData()
		if err := json.Unmarshal(b, &req); err != nil {
			ctx.JSON(400, gin.H{"error": "invalid json"})
			return
		}
	} else {
		if err := ctx.ShouldBind(&req); err != nil {
			b, _ := ctx.GetRawData()
			if err2 := json.Unmarshal(b, &req); err2 != nil {
				ctx.JSON(400, gin.H{"error": "invalid json"})
				return
			}
		}
	}
	out, err := getExercise(ctx, req)

	// 兜底假数据
	if err != nil {
		out = buildDiagnoseExercise(req)
	}
	ctx.JSON(200, out)
}

func getExercise(ctx *gin.Context, request model.GetDiagnoseExerciseRequest) (model.GetExerciseResponse, error) {
	title := request.Title
	desc := request.Description
	if title == "" {
		title = "全等三角形判定概念"
	}

	// 定义 JSON Schema
	jsonSchema := map[string]any{
		"name":   "exercise_response",
		"strict": true,
		"schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"GetExerciseList": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"Title":    map[string]any{"type": "string", "description": "练习主题"},
							"Concepts": map[string]any{"type": "string", "description": "核心概念说明"},
							"WarnInfo": map[string]any{"type": "string", "description": "注意事项提示"},
							"Questions": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"Title": map[string]any{"type": "string", "description": "题目内容"},
										"Select": map[string]any{
											"type": "array",
											"items": map[string]any{
												"type": "object",
												"properties": map[string]any{
													"Parse": map[string]any{"type": "string", "description": "选项文本内容"},
												},
												"required":             []string{"Parse"},
												"additionalProperties": false,
											},
											"description": "选项列表",
										},
										"CorrectAnswer": map[string]any{"type": "string", "description": "正确答案（必须与某个选项的Parse值完全一致）"},
									},
									"required":             []string{"Title", "Select", "CorrectAnswer"},
									"additionalProperties": false,
								},
							},
						},
						"required":             []string{"Title", "Concepts", "WarnInfo", "Questions"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"GetExerciseList"},
			"additionalProperties": false,
		},
	}

	systemPrompt := `# Role
你是一位专注于中考数学概念理解的练习题生成专家，擅长设计纯概念辨析题目。

# Goal
根据给定的知识点主题和描述，生成 3-5 道高质量的概念选择题，帮助学生加深对知识点的理解。

# Core Rules（核心规则 - 必须严格遵守）

## 1. 题目类型限制
- **仅生成概念题**：考察定义、性质、判定条件的理解
- **严禁计算题**：不要出现需要计算数值的题目
- **禁止公式**：题目和选项中不能包含 LaTeX 公式（如 $...$）
- **禁止特殊符号**：严禁使用 '~', '·', '#', '$', '¥' 等符号

## 2. 选项格式要求（重要）
- **Select 数组结构**：每个选项必须是对象 {"Parse": "选项内容"}
- **禁止前缀**：选项文本中不能包含 "A.", "B.", "C.", "D." 或 "A ", "B " 等前缀
- **纯文本内容**：仅保留选项的实际内容
- **示例**：
  正确：{"Parse": "两边及其夹角对应相等"}
  错误：{"Parse": "A. 两边及其夹角对应相等"}
  错误：{"Parse": "A 两边及其夹角对应相等"}

## 3. 正确答案匹配
- **CorrectAnswer** 必须与某个选项的 **Parse 值完全一致**（字符串精确匹配）
- 示例：
  如果 Select 为 [{"Parse": "可以判定"}, {"Parse": "不能判定"}]
  则 CorrectAnswer 必须是 "可以判定" 或 "不能判定"

## 4. 题目设计原则
- **聚焦概念辨析**：重点考察学生对定义、性质、条件的理解
- **避免歧义**：每道题只有一个明确的正确答案
- **难度适中**：符合中考难度，不过于简单也不过于刁钻
- **贴近主题**：所有题目必须围绕给定的主题和描述

# Output Structure（输出结构）

返回的 JSON 必须严格遵守以下结构：

{
  "GetExerciseList": [
    {
      "Title": "知识点主题（与输入的 title 相关）",
      "Concepts": "核心概念说明（30-50字，总结该知识点的关键要素）",
      "WarnInfo": "注意事项（20-30字，提醒学生答题时的注意点）",
      "Questions": [
        {
          "Title": "题目内容（清晰完整的问题描述）",
          "Select": [
            {"Parse": "选项1内容"},
            {"Parse": "选项2内容"},
            {"Parse": "选项3内容"},
            {"Parse": "选项4内容"}
          ],
          "CorrectAnswer": "选项X内容（必须与某个 Parse 值完全一致）"
        }
      ]
    }
  ]
}

# Example Output（示例输出）

{
  "GetExerciseList": [
    {
      "Title": "全等三角形判定",
      "Concepts": "全等三角形的判定方法包括SSS、SAS、ASA、AAS和HL，关键是识别对应边和对应角的关系",
      "WarnInfo": "注意区分判定条件中边和角的位置关系",
      "Questions": [
        {
          "Title": "下列条件中，不能判定两个三角形全等的是",
          "Select": [
            {"Parse": "三条边对应相等"},
            {"Parse": "两边及其夹角对应相等"},
            {"Parse": "两角及其夹边对应相等"},
            {"Parse": "三个角对应相等"}
          ],
          "CorrectAnswer": "三个角对应相等"
        },
        {
          "Title": "判定两个直角三角形全等的特有方法是",
          "Select": [
            {"Parse": "斜边和一条直角边对应相等"},
            {"Parse": "两个锐角对应相等"},
            {"Parse": "一个锐角和一条边对应相等"},
            {"Parse": "两条直角边对应相等"}
          ],
          "CorrectAnswer": "斜边和一条直角边对应相等"
        }
      ]
    }
  ]
}

# Final Reminder
- 返回纯 JSON 字符串，不要包含 Markdown 代码块标记
- 所有字段类型必须与 Schema 定义完全匹配
- Select 必须是对象数组，每个对象包含 Parse 字段
- CorrectAnswer 必须与某个 Select[].Parse 值完全一致`

	prompt := fmt.Sprintf("请为以下知识点生成练习题：\n主题：%s\n描述：%s\n\n请生成3-5道高质量的概念选择题。", title, desc)

	payload := map[string]any{
		"model": "gemini-3-flash",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": jsonSchema,
		},
	}

	body, _ := json.Marshal(payload)
	reqUp, _ := http.NewRequestWithContext(ctx.Request.Context(), "POST", "http://ai-service.tal.com/openai-compatible/v1/chat/completions", bytes.NewReader(body))

	appID := "300000281"
	appKey := "2be1698da309b52eb807e9ac2d6a4ff1"
	if appID != "" && appKey != "" {
		reqUp.Header.Set("Authorization", "Bearer "+appID+":"+appKey)
	}
	reqUp.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(reqUp)
	if err != nil {
		return model.GetExerciseResponse{}, err
	}
	defer resp.Body.Close()

	var aiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return model.GetExerciseResponse{}, err
	}
	if len(aiResp.Choices) == 0 || aiResp.Choices[0].Message.Content == "" {
		return model.GetExerciseResponse{}, &json.SyntaxError{}
	}

	raw := utils.ExtractJSONFromContent(aiResp.Choices[0].Message.Content)

	var out model.GetExerciseResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		fmt.Println("解析ai内容为json失败：", err)
		return model.GetExerciseResponse{}, err
	}
	return out, nil
}

func deriveExerciseRequestFromContent(content string) model.GetDiagnoseExerciseRequest {
	raw := utils.ExtractJSONFromContent(content)
	var simple struct {
		Title       string `json:"Title"`
		Description string `json:"Description"`
	}
	if err := json.Unmarshal([]byte(raw), &simple); err == nil {
		return model.GetDiagnoseExerciseRequest{Title: strings.TrimSpace(simple.Title), Description: strings.TrimSpace(simple.Description)}
	}
	var diag struct {
		KnowledgeMasteryAnalysis []struct {
			Lamp     string `json:"lamp"`
			Category string `json:"category"`
			Analysis string `json:"analysis"`
		} `json:"knowledge_mastery_analysis"`
		OverallDiagnosis struct {
			StrategicSuggestion string `json:"strategic_suggestion"`
		} `json:"overall_diagnosis"`
	}
	if err := json.Unmarshal([]byte(raw), &diag); err == nil {
		title := ""
		desc := strings.TrimSpace(diag.OverallDiagnosis.StrategicSuggestion)
		for _, a := range diag.KnowledgeMasteryAnalysis {
			if a.Lamp == "蓝灯" {
				title = strings.TrimSpace(a.Category)
				if strings.TrimSpace(a.Analysis) != "" {
					desc = strings.TrimSpace(a.Analysis)
				}
				break
			}
		}
		return model.GetDiagnoseExerciseRequest{Title: title, Description: desc}
	}
	return model.GetDiagnoseExerciseRequest{}
}

func buildDiagnoseExercise(req model.GetDiagnoseExerciseRequest) model.GetExerciseResponse {
	title := req.Title
	if title == "" {
		title = "全等三角形判定概念"
	}
	mk := func(opts []string) []model.Select {
		out := make([]model.Select, 0, len(opts))
		for _, s := range opts {
			out = append(out, model.Select{Parse: s})
		}
		return out
	}
	qs := []model.Question{
		{Title: "下列判定中正确的是？", Select: mk([]string{"SSS", "SAS", "ASA", "AAA"}), CorrectAnswer: "SAS"},
		{Title: "两边及夹角相等可判定全等吗？", Select: mk([]string{"可以", "不可以", "取决于角度", "无法判断"}), CorrectAnswer: "可以"},
		{Title: "关于解析几何的说法正确的是？", Select: mk([]string{"判别式只用于二次方程", "韦达定理可用于系数关系", "抛物线无焦点", "椭圆离心率恒为1"}), CorrectAnswer: "韦达定理可用于系数关系"},
	}
	return model.GetExerciseResponse{GetExerciseList: []model.GetExerciseList{{Title: title, Concepts: "概念：" + title, WarnInfo: "注意：仔细审题，流程化表达", Questions: qs}}}
}
