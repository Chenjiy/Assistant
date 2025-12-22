package diagnose

import (
    "bytes"
    "diagnose-server/model"
    "diagnose-server/utils"
    "encoding/json"
    "github.com/gin-gonic/gin"
    "io"
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
	systemPrompt := "你是中考数学练习生成助手。只生成概念题，不生成计算题；避免公式与特殊符号（禁止 '~','·','#','$','¥'）；选项仅为内容字符串，不能包含 'A.' 'B.' 前缀；'CorrectAnswer' 必须等于选项文本之一。请根据主题与描述生成练习，并严格输出为 GetExerciseResponse。"
	prompt := "主题：" + title + "；描述：" + desc

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
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "GetExerciseResponse",
				"strict": true,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"GetExerciseList": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"Title":    map[string]any{"type": "string"},
									"Concepts": map[string]any{"type": "string"},
									"WarnInfo": map[string]any{"type": "string"},
									"Questions": map[string]any{
										"type":  "array",
										"items": map[string]any{"$ref": "#/definitions/Question"},
									},
								},
								"required": []string{"Title", "Concepts", "WarnInfo", "Questions"},
							},
						},
					},
					"required": []string{"GetExerciseList"},
					"definitions": map[string]any{
						"Question": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"Title":         map[string]any{"type": "string"},
								"Select":        map[string]any{"type": "array", "items": map[string]any{"$ref": "#/definitions/Select"}},
								"CorrectAnswer": map[string]any{"type": "string"},
							},
							"required": []string{"Title", "Select", "CorrectAnswer"},
						},
						"Select": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"Parse": map[string]any{"type": "string"},
							},
							"required": []string{"Parse"},
						},
					},
				},
			},
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

    data, _ := io.ReadAll(resp.Body)
    var aiResp struct {
        Choices []struct {
            Message struct {
                Content string          `json:"content"`
                Parsed  json.RawMessage `json:"parsed"`
            } `json:"message"`
        } `json:"choices"`
    }
    if err := json.Unmarshal(data, &aiResp); err != nil {
        return model.GetExerciseResponse{}, err
    }
    if len(aiResp.Choices) == 0 || (aiResp.Choices[0].Message.Content == "" && len(aiResp.Choices[0].Message.Parsed) == 0) {
        return model.GetExerciseResponse{}, &json.SyntaxError{}
    }

    var out model.GetExerciseResponse
    if len(aiResp.Choices[0].Message.Parsed) > 0 {
        if err := json.Unmarshal(aiResp.Choices[0].Message.Parsed, &out); err != nil {
            return model.GetExerciseResponse{}, err
        }
    } else {
        raw := utils.ExtractJSONFromContent(aiResp.Choices[0].Message.Content)
        if err := json.Unmarshal([]byte(raw), &out); err != nil {
            var old struct {
                Title     string `json:"Title"`
                Concepts  string `json:"Concepts"`
                WarnInfo  string `json:"WarnInfo"`
                Questions []struct {
                    Title         string   `json:"Title"`
                    Select        []string  `json:"Select"`
                    CorrectAnswer string   `json:"CorrectAnswer"`
                } `json:"Questions"`
            }
            if err2 := json.Unmarshal([]byte(raw), &old); err2 != nil {
                return model.GetExerciseResponse{}, err
            }
            mk := func(opts []string) []model.Select {
                res := make([]model.Select, 0, len(opts))
                for _, s := range opts {
                    res = append(res, model.Select{Parse: s})
                }
                return res
            }
            qs := make([]model.Question, 0, len(old.Questions))
            for _, q := range old.Questions {
                qs = append(qs, model.Question{Title: q.Title, Select: mk(q.Select), CorrectAnswer: q.CorrectAnswer})
            }
            out = model.GetExerciseResponse{GetExerciseList: []model.GetExerciseList{{Title: old.Title, Concepts: old.Concepts, WarnInfo: old.WarnInfo, Questions: qs}}}
        }
    }
    derived := deriveExerciseRequestFromContent(aiResp.Choices[0].Message.Content)
    if len(out.GetExerciseList) > 0 {
        if strings.TrimSpace(derived.Title) != "" { out.GetExerciseList[0].Title = derived.Title }
        if strings.TrimSpace(derived.Description) != "" { out.GetExerciseList[0].Concepts = "概念：" + derived.Description }
    }
    return out, nil
}

func deriveExerciseRequestFromContent(content string) model.GetDiagnoseExerciseRequest {
    raw := utils.ExtractJSONFromContent(content)
    var simple struct { Title string `json:"Title"`; Description string `json:"Description"` }
    if err := json.Unmarshal([]byte(raw), &simple); err == nil {
        return model.GetDiagnoseExerciseRequest{Title: strings.TrimSpace(simple.Title), Description: strings.TrimSpace(simple.Description)}
    }
    var diag struct {
        KnowledgeMasteryAnalysis []struct { Lamp string `json:"lamp"`; Category string `json:"category"`; Analysis string `json:"analysis"` } `json:"knowledge_mastery_analysis"`
        OverallDiagnosis struct { StrategicSuggestion string `json:"strategic_suggestion"` } `json:"overall_diagnosis"`
    }
    if err := json.Unmarshal([]byte(raw), &diag); err == nil {
        title := ""; desc := strings.TrimSpace(diag.OverallDiagnosis.StrategicSuggestion)
        for _, a := range diag.KnowledgeMasteryAnalysis { if a.Lamp == "蓝灯" { title = strings.TrimSpace(a.Category); if strings.TrimSpace(a.Analysis) != "" { desc = strings.TrimSpace(a.Analysis) }; break } }
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
