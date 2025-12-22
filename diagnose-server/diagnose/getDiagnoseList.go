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

func GetDiagnoseList(ctx *gin.Context) {
	var req model.GetDiagnoseListRequest
	ct := ctx.GetHeader("Content-Type")
	if strings.Contains(ct, "application/json") {
		b, _ := ctx.GetRawData()
		if err := json.Unmarshal(b, &req); err != nil {
			ctx.JSON(400, gin.H{"error": "invalid json"})
			return
		}
	} else if strings.Contains(ct, "application/x-www-form-urlencoded") || strings.Contains(ct, "multipart/form-data") {
		_ = ctx.Request.ParseForm()
		arr := ctx.PostFormArray("ImgLink")
		if len(arr) == 0 {
			v := ctx.PostForm("ImgLink")
			if v != "" {
				arr = strings.Split(v, ",")
			}
		}
		req.ImgLink = arr
	} else {
		if err := ctx.ShouldBind(&req); err != nil {
			b, _ := ctx.GetRawData()
			if err2 := json.Unmarshal(b, &req); err2 != nil {
				ctx.JSON(400, gin.H{"error": "invalid json"})
				return
			}
		}
	}
	if len(req.ImgLink) == 0 {
		ctx.JSON(400, gin.H{"error": "missing ImgLink"})
		return
	}
	out, err := aiDiagnose(ctx, req.ImgLink)

	// 兜底假数据
	if err != nil {
		out = buildDiagnose(req.ImgLink)
	}
	ctx.JSON(200, out)
}

func aiDiagnose(ctx *gin.Context, imgs []string) (model.GetDiagnoseListResponse, error) {
	// aiRespSet := "返回结果按照下面的结构体返回, type GetDiagnoseListResponse struct {\n\tReport        Report         `json:\"Report\"`\n\tScoreSpace    int64          `json:\"ScoreSpace\"`\n\tDiagnoseList  []DiagnoseInfo `json:\"DiagnoseList\"`\n\tFinalAnalysis []FinalAnalysis  `json:\"FinalAnalysis\"`\n}\n\ntype FinalAnalysis struct {\n\tAnalysisTitle string         `json:\"AnalysisTitle\"`\n\tAnalysisItem  []AnalysisItem `json:\"AnalysisItem\"`\n}\n\ntype AnalysisItem struct {\n\tAnalysisItemTitle string `json:\"AnalysisItemTitle\"`\n\tAnalysisItemDesp  string `json:\"AnalysisItemDesp\"`\n}\n\ntype Report struct {\n\tConclusion  string       `json:\"Conclusion\"`\n\tKSMAnalysis []CommonInfo `json:\"KSMAnalysis\"`\n\tStudyMethod []CommonInfo `json:\"StudyMethod\"`\n}\n\ntype CommonInfo struct {\n\tTitle       string `json:\"Title\"`\n\tDescription string `json:\"Description\"`\n}\n\ntype DiagnoseInfo struct {\n\tTitle       string `json:\"Title\"`\n\tDegree      string `json:\"Degree\"`\n\tStatus      int64  `json:\"Status\"`\n\tExpectScore int64  `json:\"ExpectScore\"`\n\tScore       int64  `json:\"Score\"`\n\tDescription string `json:\"Description\"`\n\tIsDiagnose  bool   `json:\"IsDiagnose\"`\n}\n\ntype GetDiagnoseExerciseRequest struct {\n\tTitle       string `json:\"Title\"`\n\tDescription string `json:\"Description\"`\n}\n\ntype GetExerciseResponse struct {\n\tTitle     string     `json:\"Title\"`\n\tConcepts  string     `json:\"Concepts\"`\n\tWarnInfo  string     `json:\"WarnInfo\"`\n\tQuestions []Question `json:\"Questions\"`\n}\n\ntype Question struct {\n\tTitle         string   `json:\"Title\"`\n\tSelect        []string `json:\"Select\"`\n\tCorrectAnswer string   `json:\"CorrectAnswer\"`\n}"
systemPrompt := `角色：资深中考数学诊断助手
目标：基于用户提供的试卷图片，生成严格 JSON，完全符合 GetDiagnoseListResponse。
输出要求：
1. 仅输出 JSON，无 Markdown 代码块。
2. 严禁幻觉；无法识别时给出委婉提示但不编造。

生成规则：
- 题目识别与聚合：识别题号、题干要点、原始分值与实得分；按“二级知识点”聚合并计算掌握度=实得分总/原始分总。
- ScoreSpace：所有蓝灯知识点题目的(总分-实得分)之和，值<28。
- ReportConclusion：120-130字，避免出现“红绿灯”“得分率”等词。
- AnalysisInfoList：每项包含
  KnowledgeTitle、Degree、Status(蓝=1 红=2 绿=3)、ExpectScore(≤Score)、Description、Score(字符串)、
  OriginProblem(数组：ProblemTitle、ProblemNumber)、isDiagnose。
  规则：从所有蓝灯中选择 Score 最大的一项将 isDiagnose=true，其余为 false。
- DeepDiagnoseList：按 K/S/M 三维生成三项，含 Title、Description、ProblemNumber(题号数组)、Strategy(StrategyTitle、StrategyDesp)。
- FinalAnalysis：生成三项：
  1) 蓝灯：AnalysisTitle 为蓝灯知识点集合；AnalysisItem 三条：做模型/做对题/防失误。
  2) 红灯：AnalysisTitle 为红灯知识点集合；给出暂缓理由的 AnalysisItem。
  3) 绿灯：AnalysisTitle 为绿灯知识点集合；给出保持发挥的安抚建议的 AnalysisItem。`

	// 3. 构建 User Prompt (仅包含动态数据)
	prompt := fmt.Sprintf("请分析以下试卷图片链接，并严格按照定义的 JSON 结构返回数据：\n%s", strings.Join(imgs, "\n"))
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
				"name":   "DiagnoseResponse",
				"strict": true,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":   "DiagnoseResponse",
						"strict": true,
						"schema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"ScoreSpace":       map[string]any{"type": "integer"},
								"ReportConclusion": map[string]any{"type": "string"},
								"AnalysisInfoList": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"KnowledgeTitle": map[string]any{"type": "string"},
											"Degree":         map[string]any{"type": "string"},
											"Status":         map[string]any{"type": "integer"},
											"ExpectScore":    map[string]any{"type": "integer"},
											"Description":    map[string]any{"type": "string"},
											"Score":          map[string]any{"type": "string"},
											"OriginProblem":  map[string]any{"type": "array", "items": map[string]any{"$ref": "#/definitions/OriginProblem"}},
											"isDiagnose":     map[string]any{"type": "boolean"},
										},
										"required": []string{"KnowledgeTitle", "Degree", "Status", "ExpectScore", "Description", "Score", "OriginProblem", "isDiagnose"},
									},
								},
								"DeepDiagnoseList": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"Title":         map[string]any{"type": "string"},
											"Description":   map[string]any{"type": "string"},
											"ProblemNumber": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
											"Strategy":      map[string]any{"$ref": "#/definitions/Strategy"},
										},
										"required": []string{"Title", "Description", "ProblemNumber", "Strategy"},
									},
								},
								"FinalAnalysis": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"AnalysisTitle": map[string]any{"type": "string"},
											"AnalysisItem":  map[string]any{"type": "array", "items": map[string]any{"$ref": "#/definitions/AnalysisItem"}},
										},
										"required": []string{"AnalysisTitle", "AnalysisItem"},
									},
								},
							},
							"required": []string{"ScoreSpace", "ReportConclusion", "AnalysisInfoList", "DeepDiagnoseList", "FinalAnalysis"},
							"definitions": map[string]any{
								"AnalysisItem": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"AnalysisItemTitle": map[string]any{"type": "string"},
										"AnalysisItemDesp":  map[string]any{"type": "string"},
									},
									"required": []string{"AnalysisItemTitle", "AnalysisItemDesp"},
								},
								"OriginProblem": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"ProblemTitle":  map[string]any{"type": "string"},
										"ProblemNumber": map[string]any{"type": "integer"},
									},
									"required": []string{"ProblemTitle", "ProblemNumber"},
								},
								"Strategy": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"StrategyTitle": map[string]any{"type": "string"},
										"StrategyDesp":  map[string]any{"type": "string"},
									},
									"required": []string{"StrategyTitle", "StrategyDesp"},
								},
							},
						},
					},

					"required": []string{"Report", "ScoreSpace", "DiagnoseList", "FinalAnalysis"},
					"definitions": map[string]any{
						"CommonInfo": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"Title":       map[string]any{"type": "string"},
								"Description": map[string]any{"type": "string"},
							},
							"required": []string{"Title", "Description"},
						},
					},
				},
			},
		},
	}
	// 覆盖 response_format 以匹配最新的 GetDiagnoseListResponse 定义

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
		return model.GetDiagnoseListResponse{}, err
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
		return model.GetDiagnoseListResponse{}, err
	}
	if len(aiResp.Choices) == 0 || aiResp.Choices[0].Message.Content == "" {
		return model.GetDiagnoseListResponse{}, ioErr()
	}

	// 从ai返回的内容中解析json字符串
	raw := utils.ExtractJSONFromContent(aiResp.Choices[0].Message.Content)

	var diagnoseResp model.GetDiagnoseListResponse
	if err := json.Unmarshal([]byte(raw), &diagnoseResp); err != nil {
		fmt.Println("解析ai内容为json失败：", err)
		return model.GetDiagnoseListResponse{}, err
	}

	return diagnoseResp, nil
}
func ioErr() error { return &json.SyntaxError{} }

func buildDiagnose(imgs []string) model.GetDiagnoseListResponse {
	infos := []model.AnalysisInfo{
		{KnowledgeTitle: "全等判定", Degree: "75%", Status: 1, ExpectScore: 6, Description: "判定条件识别不牢固，易在边角边与边边边混淆", Score: "6", OriginProblem: []model.OriginProblem{{ProblemTitle: "全等三角形判定", ProblemNumber: 12}}, IsDiagnose: true},
		{KnowledgeTitle: "二次函数图像", Degree: "42%", Status: 2, ExpectScore: 5, Description: "读图与性质套用不熟，顶点坐标与对称轴易错", Score: "5", OriginProblem: []model.OriginProblem{{ProblemTitle: "二次函数图像阅读", ProblemNumber: 18}}, IsDiagnose: false},
		{KnowledgeTitle: "基础计算", Degree: "95%", Status: 3, ExpectScore: 0, Description: "保持稳定发挥，按既有节奏作答即可", Score: "0", OriginProblem: []model.OriginProblem{{ProblemTitle: "分式运算", ProblemNumber: 3}}, IsDiagnose: false},
	}
	deep := []model.DeepDiagnose{
		{KSMTitle: "K (Knowledge)", KSMDescription: "概念边界未清晰，关键判定语义混淆，易造成误判。", ProblemNumber: []int64{12}, Strategy: model.Strategy{StrategyTitle: "概念回扫", StrategyDesp: "用费曼学习法复述判定条件，校对关键字。"}},
		{KSMTitle: "S (Skill)", KSMDescription: "演算存在跳步与漏写，草稿不成体系，导致细节丢分。", ProblemNumber: []int64{18}, Strategy: model.Strategy{StrategyTitle: "分步检查", StrategyDesp: "每写三行停 2 秒核对符号与数字。"}},
		{KSMTitle: "M (Mindset)", KSMDescription: "遇长题急于求成，审题未充分，压力下判断失准。", ProblemNumber: []int64{}, Strategy: model.Strategy{StrategyTitle: "审题减速", StrategyDesp: "先写下“这题我先不求快”，按读—划—列执行。"}},
	}
	final := []model.FinalAnalysis{
		{AnalysisTitle: "全等三角形&几何", AnalysisItem: []model.AnalysisItem{
			{AnalysisItemTitle: "做模型", AnalysisItemDesp: "建议 15 分钟回顾具体的模型/知识点关系，画出条件与结论的映射。"},
			{AnalysisItemTitle: "做对题", AnalysisItemDesp: "完成 3 道典型题并写清解题判定条件，标注每步依据。"},
			{AnalysisItemTitle: "防失误", AnalysisItemDesp: "总结 2 条在草稿本上的具体预防动作，明确易错触发点。"},
		}},
		{AnalysisTitle: "立体几何&暂缓", AnalysisItem: []model.AnalysisItem{{AnalysisItemTitle: "暂缓理由", AnalysisItemDesp: "步骤长、模型不熟，短期投入产出比低，先集中在高性价比点。"}}},
		{AnalysisTitle: "基础计算&保稳", AnalysisItem: []model.AnalysisItem{{AnalysisItemTitle: "保持发挥", AnalysisItemDesp: "无需额外刷题，保持节奏即可，关注进度与稳定性。"}}},
	}
	return model.GetDiagnoseListResponse{
		ScoreSpace:       12,
		ReportConclusion: "整体状态稳中有升，建议把练习时间集中在熟悉度较高但仍可提分的板块，先以模型回顾和判定条件梳理为主，穿插短时巩固，避免拉长战线导致注意力涣散。",
		AnalysisInfoList: infos,
		DeepDiagnoseList: deep,
		FinalAnalysis:    final,
	}
}
