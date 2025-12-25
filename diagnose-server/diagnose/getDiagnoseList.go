package diagnose

import (
	"bytes"
	"diagnose-server/model"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"sort"
	"strings"
)

// ==========================================
// Phase 1: 视觉提取层结构体 (Vision Layer)
// ==========================================
type RawQuestion struct {
	ID        int      `json:"id"`
	No        string   `json:"no"`
	Txt       string   `json:"txt"`
	Err       bool     `json:"err"`
	Score     int      `json:"sc"`
	MaxScore  int      `json:"max"`
	Points    []string `json:"pts"` // 标准章节 (用于聚合，如 "方程与不等式")
	Kt        string   `json:"kt"`  // [新增] 具体细分考点 (如 "判别式"，用于详情展示)
	Sug       string   `json:"sug"`
	ErrReason string   `json:"err_reason"`
}

type Step1Response struct {
	Paper []RawQuestion `json:"paper"`
}

// ==========================================
// Phase 2: 逻辑聚合层结构体 (Logic Layer)
// ==========================================
type KnowledgeStat struct {
	Title       string
	TotalScore  int
	TotalMax    int
	QuestionRef []RawQuestion
}

// ==========================================
// Phase 3: 诊断生成层结构体 (Gen Layer)
// ==========================================
type Step3Input struct {
	Stats        []SimpleStat `json:"stats"`
	TotalScore   int          `json:"total_score"`
	TotalMax     int          `json:"total_max"`
	BlueScoreGap int          `json:"blue_score_gap"`
}

type SimpleStat struct {
	Name    string   `json:"name"`
	Lamp    string   `json:"lamp"`
	Rate    string   `json:"rate"`
	RefQues []string `json:"ref_ques"`
}

// [中间态] 用于精准接收 LLM 的 PascalCase 输出
type Step3OutputLocal struct {
	Conclusion       string               `json:"Conclusion"`
	KnowledgeDesc    map[string]string    `json:"KnowledgeDesc"`
	DeepDiagnoseList []DeepDiagnoseLocal  `json:"DeepDiagnoseList"`
	FinalAnalysis    []FinalAnalysisLocal `json:"FinalAnalysis"`
}

type DeepDiagnoseLocal struct {
	Title         string  `json:"Title"`
	Description   string  `json:"Description"`
	ProblemNumber []int64 `json:"ProblemNumber"`
	Strategy      struct {
		StrategyTitle string `json:"StrategyTitle"`
		StrategyDesp  string `json:"StrategyDesp"`
	} `json:"Strategy"`
}

type FinalAnalysisLocal struct {
	AnalysisTitle string `json:"AnalysisTitle"`
	AnalysisItem  []struct {
		AnalysisItemTitle string `json:"AnalysisItemTitle"`
		AnalysisItemDesp  string `json:"AnalysisItemDesp"`
	} `json:"AnalysisItem"`
}

// ==========================================
// 主入口
// ==========================================

func GetDiagnoseList(ctx *gin.Context) {
	var req model.GetDiagnoseListRequest
	if err := bindRequest(ctx, &req); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if len(req.ImgLink) == 0 {
		ctx.JSON(400, gin.H{"error": "missing ImgLink"})
		return
	}

	// 2. Phase 1: 视觉提取 (增加 kt 字段 & 强制清洗 LaTeX & 禁表格)
	rawPaper, err := step1VisionExtract(ctx, req.ImgLink)
	if err != nil {
		fmt.Printf("Step 1 Vision Error: %v\n", err)
		ctx.JSON(500, gin.H{"error": "AI vision analysis failed", "detail": err.Error()})
		return
	}

	// 3. Phase 2: 逻辑聚合 (使用 pts 聚合，kt 透传)
	aggData, statsMap, step3In := step2LogicAggregate(rawPaper)

	// 4. Phase 3: 生成诊断
	textResp, err := step3GenerateReport(ctx, step3In)
	if err != nil {
		fmt.Printf("Step 3 Gen Error: %v\n", err)
		textResp = fallbackStep3(step3In)
	}

	// 5. 组装最终 Response
	finalResp := assembleFinalResponse(aggData, statsMap, textResp, int64(step3In.BlueScoreGap))
	
	ctx.JSON(200, finalResp)
}

// ==========================================
// 辅助函数实现
// ==========================================

func bindRequest(ctx *gin.Context, req *model.GetDiagnoseListRequest) error {
	ct := ctx.GetHeader("Content-Type")
	if strings.Contains(ct, "application/json") {
		return ctx.ShouldBindJSON(req)
	}
	_ = ctx.Request.ParseForm()
	arr := ctx.PostFormArray("ImgLink")
	if len(arr) == 0 {
		v := ctx.PostForm("ImgLink")
		if v != "" {
			arr = strings.Split(v, ",")
		}
	}
	req.ImgLink = arr
	return nil
}

// Step 1: 视觉提取
func step1VisionExtract(ctx *gin.Context, imgs []string) ([]RawQuestion, error) {
	// =================================================================================
	// 核心优化: 
	// 1. One-Shot 示例: 给出标准 JSON 样例，防止模型画表格。
	// 2. 字段分离: kt 存细节，pts 存章节。
	// =================================================================================
	promptText := `你是一个专业的阅卷助手。这是一套**老师已经用红笔批改过**的试卷。
请根据试卷上的**印刷体题目**和**手写批改痕迹**提取信息。

**！！！严禁输出 Markdown 表格！！！**
**！！！严禁使用表格符号 '|' ！！！**
必须按照json_schema格式输出。

**核心识别规则**:
1. **题号**: 找到题目序号。
2. **得分(sc)**: 找红色手写分数。红钩(√)=满分；红叉(×)=0分；半钩/问号=部分分。
3. **满分(max)**: 括号内分值。
4. **错因(err_reason)**: 观察红笔圈画位置，简述错误特征(如"答案被圈出", "计算步骤划掉", "辅助线错误")。
5. **题干(txt)**: 
   - 提取前20字。
   - **特殊要求**: 如果包含数学公式，请尽量使用**纯文本描述**或确保 LaTeX 反斜杠正确转义 (例如用 "\\" 代替 "\")，防止 JSON 解析失败。
   - 示例: "若根号x有意义..." 而不是 "若 $\sqrt{x}$..."。

6. **知识点(pts) - 强制章节归类**:
   **pts 数组中只能包含以下 13 个标准章节名称之一** (严禁使用其他词汇，严禁细分)：
   - "数与式" (含后面名词相关即归为数与式，如实数, 整式, 分式, 二次根式)
   - "方程与不等式" (含一元一次/二次方程, 方程组, 不等式相关即归为方程与不等式)
   - "一次函数" (含正比例，一次函数相关即归为一次函数)
   - "反比例函数" (含反比例函数相关即归为反比例函数)
   - "二次函数" (含二次函数相关即归为二次函数)
   - "几何初步与三角形" (含线段, 角, 全等, 等腰, 直角相关即归为几何初步与三角形)
   - "相似与变换" (含相似, 位似, 平移, 旋转, 折叠相关即归为相似与变换)
   - "四边形" (含平行四边形, 矩形, 菱形, 正方形相关即归为四边形)
   - "圆" (含圆相关即归为圆)
   - "解直角三角形" (含锐角三角函数, 应用相关即归为解直角三角形)
   - "统计与概率" (含统计与概率相关即归为统计与概率)
   - "综合实践" (跨章节大题)

**字段要求**:
- id: Integer
- no: String
- txt: String (题干)
- sc: Integer
- max: Integer
- err: Boolean
- pts: Array<String> (仅限上述13个标准词汇)
- sug: String
- err_reason: String

**输出格式示例 (严格模仿)**:
{
  "paper": [
    {
      "id": 1,
      "no": "1",
      "txt": "若根号x-1有意义...",
      "sc": 3,
      "max": 3,
      "err": false,
      "pts": ["数与式"],
      "kt": "二次根式有意义的条件",
      "sug": "掌握良好。",
      "err_reason": ""
    }
  ]
}`

	content := []map[string]any{
		{"type": "text", "text": promptText},
	}
	for _, img := range imgs {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": img},
		})
	}

	jsonSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paper": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":         map[string]any{"type": "integer"},
						"no":         map[string]any{"type": "string"},
						"txt":        map[string]any{"type": "string"},
						"sc":         map[string]any{"type": "integer"},
						"max":        map[string]any{"type": "integer"},
						"err":        map[string]any{"type": "boolean"},
						"pts":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"kt":         map[string]any{"type": "string"},
						"sug":        map[string]any{"type": "string"},
						"err_reason": map[string]any{"type": "string"},
					},
					"required": []string{"id", "no", "txt", "sc", "max", "err", "pts", "kt", "sug", "err_reason"},
				},
			},
		},
		"required": []string{"paper"},
	}

	messages := []map[string]any{
		{"role": "user", "content": content},
	}

	fmt.Println(">>> Step 1 Request: Sending images to Vision Model (Force JSON & Dual-Field)...")

	respStr, err := callLLM(ctx, messages, jsonSchema, "gemini-3-flash", 0.1)
	if err != nil {
		return nil, err
	}

	cleanJson := repairJSON(respStr)
	fmt.Printf("<<< Step 1 Response (Raw):\n%s\n", cleanJson)

	if strings.HasPrefix(cleanJson, "[") {
		var paper []RawQuestion
		if err := json.Unmarshal([]byte(cleanJson), &paper); err != nil {
			return nil, fmt.Errorf("json parse array failed: %v", err)
		}
		return paper, nil
	} else {
		var resp Step1Response
		if err := json.Unmarshal([]byte(cleanJson), &resp); err != nil {
			return nil, fmt.Errorf("json parse object failed: %v", err)
		}
		return resp.Paper, nil
	}
}


// Step 2: 逻辑聚合
func step2LogicAggregate(paper []RawQuestion) ([]model.AnalysisInfo, map[string]*KnowledgeStat, Step3Input) {
	stats := make(map[string]*KnowledgeStat)
	totalScore := 0
	totalMax := 0

	for _, q := range paper {
		totalScore += q.Score
		totalMax += q.MaxScore

		// 聚合逻辑：使用 pts (标准章节) 作为 Key
		kps := q.Points
		if len(kps) == 0 {
			kps = []string{"综合问题"}
		}
		kp := kps[0] 

		if _, ok := stats[kp]; !ok {
			stats[kp] = &KnowledgeStat{Title: kp}
		}
		s := stats[kp]
		s.TotalScore += q.Score
		s.TotalMax += q.MaxScore
		s.QuestionRef = append(s.QuestionRef, q)
	}

	var infos []model.AnalysisInfo
	var simpleStats []SimpleStat
	blueScoreGap := 0

	for title, s := range stats {
		if s.TotalMax == 0 {
			s.TotalMax = 1
		}
		ratio := float64(s.TotalScore) / float64(s.TotalMax)

		var status int64
		var lampStr string

		if ratio > 0.90 {
			status = 3
			lampStr = "绿灯"
		} else if ratio >= 0.50 {
			status = 1
			lampStr = "蓝灯"
		} else {
			status = 2
			lampStr = "红灯"
		}

		var expect int64
		if status == 1 {
			expect = int64(float64(s.TotalMax) * 0.9)
			blueScoreGap += (s.TotalMax - s.TotalScore)
		} else if status == 2 {
			expect = int64(float64(s.TotalMax) * 0.6)
		} else {
			expect = int64(s.TotalScore)
		}

		// 构建 OriginProblem (使用 make 避免 null)
		originProbs := make([]model.OriginProblem, 0)
		var refQuesTxts []string
		
		for _, q := range s.QuestionRef {
			// 前端展示：题号 + 题干 (这里可以考虑把 kt 拼进去，但为了保持简洁暂时只用 Txt)
			originProbs = append(originProbs, model.OriginProblem{
				ProblemTitle:  q.Txt, // 前端展示题干
				ProblemNumber: int64(q.ID),
			})

			// 给 LLM 的上下文：包含【具体考点 kt】，帮助它生成精准诊断
			detail := fmt.Sprintf("题%s(得%d/%d)-[考点:%s]", q.No, q.Score, q.MaxScore, q.Kt)
			if q.Err && q.ErrReason != "" {
				detail += fmt.Sprintf("-[错因:%s]", q.ErrReason)
			}
			refQuesTxts = append(refQuesTxts, detail)
		}

		infos = append(infos, model.AnalysisInfo{
			KnowledgeTitle: title, // 标准章节名
			Degree:         fmt.Sprintf("%d%%", int(ratio*100)),
			Status:         status,
			ExpectScore:    expect,
			Score:          fmt.Sprintf("%d", s.TotalMax),
			OriginProblem:  originProbs,
			IsDiagnose:     false,
		})

		simpleStats = append(simpleStats, SimpleStat{
			Name:    title,
			Lamp:    lampStr,
			Rate:    fmt.Sprintf("%d%%", int(ratio*100)),
			RefQues: refQuesTxts,
		})
	}

	// 智能排序
	sort.Slice(infos, func(i, j int) bool {
		sI := stats[infos[i].KnowledgeTitle]
		sJ := stats[infos[j].KnowledgeTitle]
		ratioI := float64(sI.TotalScore) / float64(sI.TotalMax)
		ratioJ := float64(sJ.TotalScore) / float64(sJ.TotalMax)

		prioI := getLampPriority(infos[i].Status)
		prioJ := getLampPriority(infos[j].Status)
		if prioI != prioJ {
			return prioI < prioJ
		}

		if infos[i].Status == 1 {
			idxI := float64(sI.TotalMax) * ratioI
			idxJ := float64(sJ.TotalMax) * ratioJ
			return idxI > idxJ
		} else if infos[i].Status == 2 {
			lostI := sI.TotalMax - sI.TotalScore
			lostJ := sJ.TotalMax - sJ.TotalScore
			return lostI > lostJ
		} else {
			return sI.TotalScore > sJ.TotalScore
		}
	})

	if len(infos) > 0 {
		infos[0].IsDiagnose = true
	}

	return infos, stats, Step3Input{
		Stats:        simpleStats,
		TotalScore:   totalScore,
		TotalMax:     totalMax,
		BlueScoreGap: blueScoreGap,
	}
}

func getLampPriority(status int64) int {
	if status == 1 {
		return 0
	}
	if status == 2 {
		return 1
	}
	return 2
}

// Step 3: 生成报告
func step3GenerateReport(ctx *gin.Context, input Step3Input) (Step3OutputLocal, error) {
	inputBytes, _ := json.Marshal(input)

	prompt := fmt.Sprintf(`
Role: 20年教龄数学特级教师，面对面面批。
Data: 知识点(标准章节)及**错因细节**: %s

Task: 生成诊断报告。
Rules:
1. **Tone**: 去除AI味，用"你"。结合 Data 中的"错因"细节点评。
2. **KnowledgeDesc**: Key 必须与 stats.name 逐字一致。
3. **DeepDiagnoseList (数据回溯)**: 
   - 必须包含 3 条 (K,S,M)。
   - **ProblemNumber** (必填): 必须从 Data 的 ref_ques 字段中提取对应的纯数字题号。
   - 例如: ref_ques=["题14(得0/3)..."] -> ProblemNumber=[14]。
   - **严禁返回 null 或空数组**，必须找到至少一道例题作为证据。
4. **FinalAnalysis (战略分层 - 严禁缺项)**:
   必须生成且仅生成3个建议模块，保证**FinalAnalysis**字段长度为3，严格对应数组下标：
	Task: 请严格按照以下 **JSON 模板** 进行“填空”。
	**注意**：FinalAnalysis 数组必须严格包含 3 个对象（Index 0, 1, 2），严禁增删或合并。

	**模板要求 (Template)**:

	1. **Index 0 [蓝灯区]**:
		- 目标: 掌握度 '50%'-'90%' 的模块 (若无，选分数最高的红灯模块)。
		- 动作(参考): 
			- "看概念": 建议回顾的具体知识点。
			- "做对题": 建议做的典型题型。
			- "防失误": 具体的草稿纸预防动作。

	2. **Index 1 [红灯区]**:
		- 目标: 掌握度 < 50% 的模块。
		- 动作(参考): "战略暂缓" (给出理由)。

	3. **Index 2 [绿灯区]**:
		- 目标: 掌握度 > 90% 的模块 (若无，填"暂无")。
		- 动作(参考): "保持手感" (给出建议)。
		Output Schema (Strict PascalCase):
		{
			"Conclusion": "...",
			"KnowledgeDesc": { "方程与不等式": "..." },
			"DeepDiagnoseList": [ ... ],
			"FinalAnalysis": [
				{
					"AnalysisTitle": "在这里填蓝灯知识点集合(如'方程&几何')", 
					"AnalysisItem": [
						{ "AnalysisItemTitle": "看模型", "AnalysisItemDesp": "建议 15 分钟回顾[具体模型]..." },
						{ "AnalysisItemTitle": "做对题", "AnalysisItemDesp": "建议完成 3 道[具体题型]..." },
						{ "AnalysisItemTitle": "防失误", "AnalysisItemDesp": "总结 2 条[具体动作]..." }
					]
				},
				{
					"AnalysisTitle": "在这里填红灯知识点集合", 
					"AnalysisItem": [ 
						{ "AnalysisItemTitle": "战略暂缓", "AnalysisItemDesp": "因[具体原因]建议暂缓..." } 
					]
				},
				{
					"AnalysisTitle": "在这里填绿灯知识点集合", 
					"AnalysisItem": [ 
						{ "AnalysisItemTitle": "保持手感", "AnalysisItemDesp": "无需刷题..." } 
					]
				}
			]
		}
`, string(inputBytes))

	jsonSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"Conclusion": map[string]any{"type": "string"},
			"KnowledgeDesc": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
			},
			"DeepDiagnoseList": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"Title":         map[string]any{"type": "string", "enum": []string{"K (Knowledge)", "S (Skill)", "M (Mindset)"}},
						"Description":   map[string]any{"type": "string"},
						"ProblemNumber": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
						"Strategy": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"StrategyTitle": map[string]any{"type": "string"},
								"StrategyDesp":  map[string]any{"type": "string"},
							},
							"required": []string{"StrategyTitle", "StrategyDesp"},
						},
					},
					"required": []string{"Title", "Description", "ProblemNumber", "Strategy"},
				},
				"minItems": 3,
				"maxItems": 3,
			},
			"FinalAnalysis": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"AnalysisTitle": map[string]any{"type": "string"},
						"AnalysisItem": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"AnalysisItemTitle": map[string]any{"type": "string"},
									"AnalysisItemDesp":  map[string]any{"type": "string"},
								},
								"required": []string{"AnalysisItemTitle", "AnalysisItemDesp"},
							},
						},
					},
					"required": []string{"AnalysisTitle", "AnalysisItem"},
				},
				"minItems": 3,
				"maxItems": 3,
			},
		},
		"required": []string{"Conclusion", "KnowledgeDesc", "DeepDiagnoseList", "FinalAnalysis"},
	}

	messages := []map[string]any{
		{"role": "user", "content": prompt},
	}

	fmt.Println(">>> Step 3 Request: Sending stats to Gen Model...")

	respStr, err := callLLM(ctx, messages, jsonSchema, "gemini-3-flash", 0.7)
	if err != nil {
		return Step3OutputLocal{}, err
	}

	cleanJson := repairJSON(respStr)
	fmt.Printf("<<< Step 3 Response (Raw):\n%s\n", cleanJson)

	var resp Step3OutputLocal
	if err := json.Unmarshal([]byte(cleanJson), &resp); err != nil {
		return Step3OutputLocal{}, err
	}
	return resp, nil
}

// 组装最终结果
func assembleFinalResponse(infos []model.AnalysisInfo, stats map[string]*KnowledgeStat, textResp Step3OutputLocal, scoreSpace int64) model.GetDiagnoseListResponse {
	for i := range infos {
		title := infos[i].KnowledgeTitle
		
		desc, ok := textResp.KnowledgeDesc[title]
		if !ok {
			desc, ok = textResp.KnowledgeDesc[strings.TrimSpace(title)]
		}
		if !ok {
			for k, v := range textResp.KnowledgeDesc {
				if strings.Contains(k, title) || strings.Contains(title, k) {
					desc = v
					ok = true
					break
				}
			}
		}

		if ok && desc != "" {
			infos[i].Description = desc
		} else {
			if infos[i].Status == 3 {
				infos[i].Description = "掌握得不错，细节处理很到位。"
			} else if infos[i].Status == 1 {
				infos[i].Description = "有提升空间，注意细节问题。"
			} else {
				infos[i].Description = "基础薄弱，需要重点攻克。"
			}
		}
	}

	var dList []model.DeepDiagnose
	for _, d := range textResp.DeepDiagnoseList {
		dList = append(dList, model.DeepDiagnose{
			KSMTitle:       d.Title,
			KSMDescription: d.Description,
			ProblemNumber:  d.ProblemNumber,
			Strategy: model.Strategy{
				StrategyTitle: d.Strategy.StrategyTitle,
				StrategyDesp:  d.Strategy.StrategyDesp,
			},
		})
	}

	var fList []model.FinalAnalysis
	for _, f := range textResp.FinalAnalysis {
		var items []model.AnalysisItem
		for _, item := range f.AnalysisItem {
			items = append(items, model.AnalysisItem{
				AnalysisItemTitle: item.AnalysisItemTitle,
				AnalysisItemDesp:  item.AnalysisItemDesp,
			})
		}
		fList = append(fList, model.FinalAnalysis{
			AnalysisTitle: f.AnalysisTitle,
			AnalysisItem:  items,
		})
	}

	return model.GetDiagnoseListResponse{
		ScoreSpace:       scoreSpace,
		ReportConclusion: textResp.Conclusion,
		AnalysisInfoList: infos,
		DeepDiagnoseList: dList,
		FinalAnalysis:    fList,
	}
}

// 兜底 Step3
func fallbackStep3(in Step3Input) Step3OutputLocal {
	defaultProbIDs := []int64{}
	if len(in.Stats) > 0 && len(in.Stats[0].RefQues) > 0 {
		defaultProbIDs = []int64{1} 
	}
	return Step3OutputLocal{
		Conclusion:    "阅卷完成。整体看你的基础不错，但部分知识点存在漏洞。特别是蓝灯模块，只要把细节抓起来，分数还有很大提升空间。",
		KnowledgeDesc: map[string]string{},
		DeepDiagnoseList: []DeepDiagnoseLocal{
			{Title: "K (Knowledge)", Description: "概念理解不透彻，公式应用生疏。", ProblemNumber: defaultProbIDs, Strategy: struct{StrategyTitle string `json:"StrategyTitle"`; StrategyDesp string `json:"StrategyDesp"`}{StrategyTitle: "回归课本", StrategyDesp: "重读定义与定理。"}},
			{Title: "S (Skill)", Description: "计算步骤跳跃，导致无谓失分。", ProblemNumber: defaultProbIDs, Strategy: struct{StrategyTitle string `json:"StrategyTitle"`; StrategyDesp string `json:"StrategyDesp"`}{StrategyTitle: "规范步骤", StrategyDesp: "坚持不跳步运算。"}},
			{Title: "M (Mindset)", Description: "审题急躁，遗漏关键条件。", ProblemNumber: defaultProbIDs, Strategy: struct{StrategyTitle string `json:"StrategyTitle"`; StrategyDesp string `json:"StrategyDesp"`}{StrategyTitle: "圈画关键词", StrategyDesp: "读题时强制圈画。"}},
		},
		FinalAnalysis: []FinalAnalysisLocal{
			{
				AnalysisTitle: "重点突破模块", // 蓝灯
				AnalysisItem: []struct{AnalysisItemTitle string `json:"AnalysisItemTitle"`; AnalysisItemDesp string `json:"AnalysisItemDesp"`}{
					{AnalysisItemTitle: "看模型", AnalysisItemDesp: "建议 15 分钟回顾相关公式推导。"},
					{AnalysisItemTitle: "做对题", AnalysisItemDesp: "建议完成 3 道典型错题并写清条件。"},
					{AnalysisItemTitle: "防失误", AnalysisItemDesp: "总结 2 条计算时的预防动作。"},
				},
			},
			{
				AnalysisTitle: "难点暂缓模块", // 红灯
				AnalysisItem: []struct{AnalysisItemTitle string `json:"AnalysisItemTitle"`; AnalysisItemDesp string `json:"AnalysisItemDesp"`}{
					{AnalysisItemTitle: "战略暂缓", AnalysisItemDesp: "该模块综合性强，建议暂时跳过。"},
				},
			},
			{
				AnalysisTitle: "优势保持模块", // 绿灯
				AnalysisItem: []struct{AnalysisItemTitle string `json:"AnalysisItemTitle"`; AnalysisItemDesp string `json:"AnalysisItemDesp"`}{
					{AnalysisItemTitle: "保持手感", AnalysisItemDesp: "跟进学校进度即可。"},
				},
			},
		},
	}
}

func repairJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s
	}
	start := strings.IndexAny(s, "{[")
	end := strings.LastIndexAny(s, "}]")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}

func callLLM(ctx *gin.Context, messages []map[string]any, schema any, modelName string, temp float64) (string, error) {
	payload := map[string]any{
		"model":    modelName,
		"messages": messages,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "response",
				"strict": true,
				"schema": schema,
			},
		},
		"temperature": temp,
	}

	body, _ := json.Marshal(payload)
	reqUp, _ := http.NewRequestWithContext(ctx.Request.Context(), "POST", "[http://ai-service.tal.com/openai-compatible/v1/chat/completions](http://ai-service.tal.com/openai-compatible/v1/chat/completions)", bytes.NewReader(body))

	appID := "300000281"
	appKey := "2be1698da309b52eb807e9ac2d6a4ff1"
	if appID != "" && appKey != "" {
		reqUp.Header.Set("Authorization", "Bearer "+appID+":"+appKey)
	}
	reqUp.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(reqUp)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return "", fmt.Errorf("provider error (status %d): %s", resp.StatusCode, buf.String())
	}

	var aiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return "", fmt.Errorf("decode json failed: %v", err)
	}
	if len(aiResp.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return aiResp.Choices[0].Message.Content, nil
}