package diagnose

import (
	"bytes"
	"diagnose-server/model"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
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
	systemPrompt := `1.先获取所有图片中所有题的题目以及对应题号

2.你是一位拥有 20 年经验的资深阅卷组长，精通 OCR 视觉识别与手写分值归因。你的首要任务是识别试卷图像中的红色笔迹（RGB 红色通道高亮区域），并根据阅卷习惯判定分值。[红色笔迹优先级协议]：强制过滤：忽略学生蓝/黑色笔迹，仅以红色笔迹作为判定“实得分”的唯一法定依据。全局扫描：首先定位试卷首页上方的“总分区域”（通常有大红字或“/120”字样）。2. 实得分判定逻辑 (Scoring Decision Tree)请按照以下分级逻辑判定每道题的实得分（Score）：A. 判定标志物：对勾 (√)规则：若题号旁有清晰红勾，判定该题为“全对”。分值逻辑：Score = 该题满分 (MaxScore)。注意：即便没有写具体分数，红勾即代表满分。B. 判定标志物：错号 (×) 或 半勾规则：若题号旁有错号或划线，寻找附近的红色手写数字。数字解读：若数字前带“+”：Score = +号后的数值（极少见，通常代表加分）。若仅为独立数字（如题号旁写个“2”）：Score = 该数字（代表本题实得 2 分）。若仅有错号且无数字：Score = 0。分值逻辑：Score = 识别到的红色手写数字。C. 判定标志物：大题总分（圈出的数字）规则：在填空题或解答题区域，若出现被圆圈包围的红字，通常代表该大项的总分。校验：需将该大项下各小题分值求和，与圈出总分进行对齐。3. 数据交叉校验 (Cross-Check Protocol)为防止 AI 幻觉，必须执行以下三层逻辑校验：总分守恒：统计所有小题实得分之和，必须等于（或极度接近）卷头识别到的“用户总得分”。分值合规：任何单题的 Score 严禁超过该题的 MaxScore。最终分析出试卷的每道题的题目以及题目的原分值对应的
实得分

2.对试卷所有题目进行知识点归属：必须包含 1-5 级知识点（参考中考数学大纲），将每道题映射到“二级知识点”（如：一次函数、全等三角形、实数运算）。

3.对每一类“二级知识点”进行掌握度分析，输出案例：70%；掌握度公式：该知识点下所有相关题目的（实得分总和 / 原始分总和）× 100%。

4.结合“二级知识点”和“掌握度”，将符合条件 50%<掌握度<90% 的“二级知识点”标记为蓝灯，将符合条件的 掌握度>90% 的知识点合并为一个知识点并标记为绿灯，将符合条件 掌握度<50% 的所有“二级知识点”合并为一个并标记为红灯

5. 结合试卷的所有题目和“二级知识点”和掌握度和标记，计算出符合标记为蓝灯“二级知识点”的所有题目分值的总分

6. 根据对用户得分及其水平进行整体分析，定位失分最多的知识点，给出对应建议。最后可以建议用户关注蓝灯知识点。

7. 对蓝灯（掌握度 50%-90%）的“二级知识点”进行分析：定位“临门一脚”的问题，归因逻辑：学生有基础，但存在“假懂”或“执行不到位”。分析话术建议：侧重于识别特定模型失误或判定条件遗漏。示例模板：“在 [**题号**] 中反映出你对该性质已建立初步认知，但在实际应用中对边界条件（如：SAS中的夹角要求）识别不准。这种‘差一点就对’的特征使其成为提分 ROI 最高的黄金区。”，并且获取蓝灯的所有原题题目
对红灯（掌握度 < 50%）的“二级知识点”进行分析：定位“底层缺失”的问题
归因逻辑：学生在该模块存在大面积空白，或由于综合度过高导致毫无思路。分析话术建议：侧重于模型迁移能力弱或基础公式完全遗忘，给出“暂缓”的科学依据。示例模板：“在 [**题号**] 等综合题中，你的掌握度较低，主因是多个底层模型（如：圆与相似）的复合关联能力尚未建立。现阶段死磕此类高难度压轴题产出比极低，建议战略性暂避。”并且获取红灯的所有原题题目
对绿灯（掌握度 ≥ 90%）的“二级知识点”进行分析：定位“能力达标”的状态
归因逻辑：表现极其稳定，已形成自动化反应。
分析话术建议：侧重于算法稳健、逻辑闭环。
示例模板：“你在 [**题号**] 展示的解题流程显示，你对该基础概念的提取速度与运算准确率已达标。目前已形成稳定的‘保底分’，无需额外投入刷题，跟进常规进度即可。”并且获取绿灯的所有原题题目

8. 需要进行KSM 深度诊断 (证据链分析)，从 K (知识点：侧重于模型识别不准、公式记混、概念边界模糊。)、S (解题技能：侧重于计算跳步、草稿潦草导致看错、逻辑推导不严谨。)、M (学习思维：侧重于压轴题畏难、审题急躁、考场紧张。) 三个维度对错题和试卷卷面书写进行深度闭环分析错因。
每个维度的输出约束：
格式约束：题号必须包裹为 [**题号**]。每题分析 40-60 字，解释为什么被划入该灯号。
策略匹配：
标题：xxx方法（不超过 10 字）。
内容：提供简单可执行的建议（如费曼学习法、分步检查法、慢想快做）加一句鼓励。
示例：[KSM 深度诊断] [12] 题属于 K 维度失分。你在全等判定中识别出了边角关系，但对“SSA”不成立的边界条件掌握度仅 60%，导致误选。这是你最容易拿回的黄金分。 解决方法：费曼学习法 建议明天中午尝试向同桌解释清楚 SSA 为什么不能判定全等。讲通了，这 12 分你就稳拿了。`

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
	buf, _ := io.ReadAll(resp.Body)
	fmt.Println("buf:", string(buf))
	var diagnoseResp model.GetDiagnoseListResponse
	if err := json.Unmarshal(buf, &diagnoseResp); err != nil {
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
