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
	aiRoleSet := "你是一个诊断高中数学试卷的助手，根据试卷图片链接生成诊断结果。"
	aiRespSet := "返回结果按照下面的结构体返回, type GetDiagnoseListResponse struct {\n\tReport        Report         `json:\"Report\"`\n\tScoreSpace    int64          `json:\"ScoreSpace\"`\n\tDiagnoseList  []DiagnoseInfo `json:\"DiagnoseList\"`\n\tFinalAnalysis []FinalAnalysis  `json:\"FinalAnalysis\"`\n}\n\ntype FinalAnalysis struct {\n\tAnalysisTitle string         `json:\"AnalysisTitle\"`\n\tAnalysisItem  []AnalysisItem `json:\"AnalysisItem\"`\n}\n\ntype AnalysisItem struct {\n\tAnalysisItemTitle string `json:\"AnalysisItemTitle\"`\n\tAnalysisItemDesp  string `json:\"AnalysisItemDesp\"`\n}\n\ntype Report struct {\n\tConclusion  string       `json:\"Conclusion\"`\n\tKSMAnalysis []CommonInfo `json:\"KSMAnalysis\"`\n\tStudyMethod []CommonInfo `json:\"StudyMethod\"`\n}\n\ntype CommonInfo struct {\n\tTitle       string `json:\"Title\"`\n\tDescription string `json:\"Description\"`\n}\n\ntype DiagnoseInfo struct {\n\tTitle       string `json:\"Title\"`\n\tDegree      string `json:\"Degree\"`\n\tStatus      int64  `json:\"Status\"`\n\tExpectScore int64  `json:\"ExpectScore\"`\n\tScore       int64  `json:\"Score\"`\n\tDescription string `json:\"Description\"`\n\tIsDiagnose  bool   `json:\"IsDiagnose\"`\n}\n\ntype GetDiagnoseExerciseRequest struct {\n\tTitle       string `json:\"Title\"`\n\tDescription string `json:\"Description\"`\n}\n\ntype GetExerciseResponse struct {\n\tTitle     string     `json:\"Title\"`\n\tConcepts  string     `json:\"Concepts\"`\n\tWarnInfo  string     `json:\"WarnInfo\"`\n\tQuestions []Question `json:\"Questions\"`\n}\n\ntype Question struct {\n\tTitle         string   `json:\"Title\"`\n\tSelect        []string `json:\"Select\"`\n\tCorrectAnswer string   `json:\"CorrectAnswer\"`\n}"
	aiStep1 := `你是一名资深中考数学阅卷专家。请分析图片链接的图片，先识别每道题目的以下信息并以 JSON 格式输出：\n
1. 题目信息： 题号、题型、分值、正确答案。\n
2. 作答情况： 学生答案、得分、正误判断。\n
3. 知识点归属： 必须包含 1-5 级知识点（参考中考数学大纲），
例如：几何 -> 三角形 -> 全等三角形 -> 全等三角形判定 -> SSS/SAS。\n
4. 错误原因分类： [概念模糊, 计算粗心, 逻辑断档, 题目理解偏差, 放弃作答]。\n
5. 难度： 0.1-1.0（1.0最难）。仅获取 JSON，确保数据严谨。

[OCR 严谨性协议 - 强制执行]：
1. 来源锚点校验：处理图片前必须识别页眉文字（如“2025年北京朝阳区一模”）。严禁使用历史对话中的试卷来源或题号。
2. 题号唯一性：仅以图片中加粗的数字标号（如 10, 15）为法定题号，忽略学生手写数字。
3. 防幻觉机制：严禁虚构当前图片中不存在的题目内容或分值。
[核心算法逻辑 - ROI 驱动]：
1. 提分潜力值 ($P_s$)：Σ 所有蓝灯区（掌握度 $[50\%, 90\%)$）题目的原始分值。
2. 预计提分 (单项展示用)：$分值 \times 0.5$（代表阅读诊断报告后的初步认知增益）。
3. 下次考试预估提升 ($G_e$)：$G_e = (\text{本次练习题分值} \times \text{正确率}) + (\sum \text{剩余蓝灯分值} \times 0.5)$。
4. 推荐指数 (后台排序用)：$\text{推荐指数} = 分值 \times (\text{掌握度} / 100)$。

[蓝灯排序准则]：
- 严格按 [推荐指数] 降序排列。
- 逻辑：优先推送那些“分值高且学生已有较好基础（最易提分）”的黄金题目。
基于提供的试卷分析 JSON，执行以下任务：
1. 计算掌握度： 统计每个二级知识点下，学生的（实得分/总分值）。
2. 灯号分配：
  - 绿灯：掌握度 > 90%。
  - 蓝灯：50% <= 掌握度 < 90%（提分重点）。
  - 红灯/灰灯：掌握度 < 50%。
3. 计算提分潜力：
  - 找出 ROI 最高的蓝灯：分值占比大且当前掌握度在 70%-85% 之间的知识点。
  - 预测得分：该知识点失分数 $\times 0.8$。
  - 计算 ROI 的关键变量： 建议引入“题目分值”与“错误类型”权重。计算公式优化为：
  $$ROI = \frac{\text{该知识点总分值} \times (1 - \text{当前掌握度})}{\text{知识点层级(难度)} \times \text{错误类型系数}}$$
  （注：粗心错误系数小，概念不清系数大）
4. 生成话术： 为每个灯生成一句简短评价（如“此路不通，暂且绕行”）。
5. 错误点描述： 针对各种灯下的错题，用一句话总结共性病灶（如：“对全等判定中的‘边角边’条件识别不准”）。

对应GetDiagnoseListResponse中DiagnoseInfo字段的返回值
按照试卷的每道题目的掌握度，分为不同的等级， - 绿灯：掌握度 > 90%。 - 蓝灯：50% <= 掌握度 < 90%（提分重点）。- 红灯/灰灯：掌握度 < 50%。
对于蓝灯的题目对应的知识点进行大致分类（不超过五类），每一类创建一个DiagnoseInfo元素，将该项元素标记为蓝灯（将DiagnoseInfo中的Status值设置为1，对应蓝灯），并给出每一类知识点的标题（对应DiagnoseInfo中的Title字段），知识点的分析（对应DiagnoseInfo中的Description字段），知识点的掌握度（对应DiagnoseInfo中的Degree字段），这张试卷的该类题目的分数总和（对应DiagnoseInfo中的Score字段），预期学好这个知识点能达到的预期分数（对应DiagnoseInfo中的ExpectScore, 该值一定<=Score的值），将创建的每一类DiagnoseInfo蓝灯元素加入GetDiagnoseListResponse.DiagnoseList中
对于红灯的题目对应的知识点进行大致分类（不超过五类），标记为红灯（将DiagnoseInfo中的Status值设置为2，对应红灯），并给出每一类知识点的标题（对应DiagnoseInfo中的Title字段），知识点的分析（对应DiagnoseInfo中的Description字段），知识点的掌握度（对应DiagnoseInfo中的Degree字段），这张试卷的该类题目的分数总和（对应DiagnoseInfo中的Score字段），预期学好这个知识点能达到的预期分数（对应DiagnoseInfo中的ExpectScore, 该值一定<=Score的值），将创建的每一类DiagnoseInfo蓝灯元素加入GetDiagnoseListResponse.DiagnoseList中
对于绿灯的题目对应的知识点进行大致分类（不超过五类），标记为绿灯（将DiagnoseInfo中的Status值设置为3，对应绿灯），并给出每一类知识点的标题（对应DiagnoseInfo中的Title字段），知识点的分析（对应DiagnoseInfo中的Description字段），知识点的掌握度（对应DiagnoseInfo中的Degree字段），这张试卷的该类题目的分数总和（对应DiagnoseInfo中的Score字段），预期学好这个知识点能达到的预期分数（对应DiagnoseInfo中的ExpectScore, 该值一定<=Score的值），将创建的每一类DiagnoseInfo蓝灯元素加入GetDiagnoseListResponse.DiagnoseList中
最后从DiagnoseInfo中，DiagnoseInfo中的Status=1的元素（蓝灯）中，在Status为1的GetDiagnoseListResponse.DiagnoseList元素中找出Score的值是最大的那项元素，将该项元素的IsDiagnose字段设置为true，其余所有元素的值均为false


任务要求：
1. 总体评价：120-130 字。结合 KSM 分布评价状态，给出整体冲刺节奏建议。该点对应GetDiagnoseListResponse中Report的Conclusion字段。
2. 潜力值展示：显示计算后的 $P_s$（蓝灯总分）。该点对应GetDiagnoseListResponse中ScoreSpace字段（该值一定小于28）。
3. 三类学习方法 (必须输出 3 项，每项 45-75 字)：该点对应GetDiagnoseListResponse中Report的StudyMethod字段。
  - 概念模糊类：匹配 [费曼学习法]，给出明天中午找同桌讲解的具体动作。该点对应GetDiagnoseListResponse中Report的StudyMethod字段的第一项的Description字段值，该项的Title字段值为“概念模糊类”。
  - 计算粗心类：匹配 [分步检查法]，要求每写三行停顿 2 秒。该点对应GetDiagnoseListResponse中Report的StudyMethod字段的第二项的Description字段值，该项的Title字段值为“计算粗心类”。
  - 心态波动类：匹配 [审题减速]，要求在草稿纸写下“这题我先不求快”。该点对应GetDiagnoseListResponse中Report的StudyMethod字段的第三项的Description字段值，该项的Title字段值为“心态波动类”。
4. 对每一道错题进行深度“病因”分析，判定错误类型（三选一）：该点对应GetDiagnoseListResponse中Report的KSMAnalysis字段
  - K (Knowledge)：概念模糊、公式记错、性质理解偏差。该点对应GetDiagnoseListResponse中Report的KSMAnalysis字段的第一项的Description字段值（分析输出：给出 20-30 字的精准分析，直接点破失分真相。），该项的Title字段值为“K (Knowledge)”。
  - S (Skill)：计算跳步、草稿凌乱、枚举漏项。该点对应GetDiagnoseListResponse中Report的KSMAnalysis字段的第二项的Description字段值（分析输出：给出 20-30 字的精准分析，直接点破失分真相。），该项的Title字段值为“S (Skill)”。
  - M (Mindset)：压轴题畏难、长文本审题缺失、考场急躁。该点对应GetDiagnoseListResponse中Report的KSMAnalysis字段的第三项的Description字段值（分析输出：给出 20-30 字的精准分析，直接点破失分真相。），该项的Title字段值为“M (Mindset)”。

5. 你是一位资深中考数学提分教练，负责为学生生成最后一份“学习建议清单”。基于本次试卷诊断和 3 分钟练习的数据，生成三个梯度的学习行动方案。
Logic & Constraints 应GetDiagnoseListResponse中FinalAnalysis字段
a. [蓝灯：最应该重点投入] (高性价比点) 对应GetDiagnoseListResponse中FinalAnalysis字段的第一个元素的AnalysisTitle字段，值为上面蓝灯题目对应知识点的抽象集合，要求字数在10个以内，例如：三角形全等&几何”。
  - 选取掌握度 50%-90% 且刚练习过的知识点。
  - 输出 3 个标准化动作：
  ① 看模型：建议 15 分钟回顾具体的模型/知识点关系。
  ② 做对题：建议完成 3 道典型题并写清解题判定条件。
  ③ 防失误：总结 2 条在草稿本上的具体预防动作。
仿照上面的标准化动作的描述，对于GetDiagnoseListResponse中FinalAnalysis字段的第一个元素的AnalysisItem，在这个元素内添加三个AnalysisItem元素（AnalysisItem的Title字段仿照“做模型”，“做对题”，“防失误”输出 3 个标准化动作；AnalysisItem的Description字段值仿照“建议 15 分钟回顾具体的模型/知识点关系”，“建议完成 3 道典型题并写清解题判定条件”，“总结 2 条在草稿本上的具体预防动作”）
对应GetDiagnoseListResponse中FinalAnalysis字段的第二个元素的AnalysisItemDesp字段值。
    
b. [红灯：建议放弃] (短期投入产出比低) 对应GetDiagnoseListResponse中第二个元素FinalAnalysis的AnalysisTitle字段，值为上面红灯题目对应知识点的抽象集合，要求字数在10个以内，例如：三角形全等&几何”。
  - 给出“暂缓”理由（如步骤长、模型不熟、得分不稳定）。对应GetDiagnoseListResponse中FinalAnalysis字段的第二个元素的AnalysisItemDesp字段值。
c. [绿灯：保持发挥] (保底分) 对应GetDiagnoseListResponse中第三个元素FinalAnalysis的AnalysisTitle字段，值为上面红灯题目对应知识点的抽象集合，要求字数在10个以内，例如：三角形全等&几何”。
  - 选取掌握度 > 90% 的基础计算或常考点。
  - 给出“无需额外刷题、跟进进度”的安抚建议。
结合“选取掌握度 > 90% 的基础计算或常考点。”和“给出“无需额外刷题、跟进进度”的安抚建议。” 给出GetDiagnoseListResponse中FinalAnalysis字段的第三个元素的AnalysisItemDesp字段值。
`
	aiStep2 := `你是学习规划师。基于提供的试卷分析 JSON，执行以下任务：
5.Question只出概念题，不需要出计算题，不带公式或者避免出现特殊符号, 以下符号都不要出现：'~','·'，'#','$','¥'；并且每个Question中Select的元素不要包含A. B. C. ,仅包含选项描述字符串即可，一定不要出现这种：“A. 最大值为2，x=3”或者“A 最大值为2，x=3”，预期应该输出以下文案：“最大值为2，x=3”，并且CorrectAnswer的值为正确答案的字符串，例如：若selcet中的["x=1","x=2","x=3","x=4"]，则CorrectAnswer的值为"x=3"。
`
	prompt := "根据以下图片链接生成诊断结果，必须返回严格JSON，链接列表：" + strings.Join(imgs, ",")
	payload := map[string]any{
		"model": "gemini-3-flash",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": aiRoleSet + aiRespSet + aiStep1 + aiStep2,
			},
			{
				"role":    "user",
				"content": prompt,
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

func buildDiagnose(imgs []string) model.GetDiagnoseListResponse {
	n := int64(len(imgs))
	dl := []model.DiagnoseInfo{
		{Title: "全等三角形判定概念", Degree: "70%", Status: 1, ExpectScore: 8, Description: "分析：基础扎实", IsDiagnose: true},
		{Title: "解析几何综合", Degree: "55%", Status: 2, ExpectScore: 10, Description: "分析：运算耐力不足", IsDiagnose: false},
		{Title: "导数与函数性质", Degree: "40%", Status: 3, ExpectScore: 12, Description: "分析：分类讨论不完整", IsDiagnose: false},
	}
	rep := model.Report{
		Conclusion:  "总体：基础较好，重点突破计算与逻辑",
		KSMAnalysis: []model.CommonInfo{{Title: "知识点", Description: "解析几何与导数为短板"}, {Title: "技能", Description: "计算准确性需提升"}, {Title: "心态", Description: "遇到繁琐运算易焦虑"}},
		StudyMethod: []model.CommonInfo{{Title: "模板化", Description: "导数分类讨论流程化"}, {Title: "限时训练", Description: "解析几何耐力题日练"}},
	}
	return model.GetDiagnoseListResponse{Report: rep, ScoreSpace: n * 15, DiagnoseList: dl}
}

func ioErr() error { return &json.SyntaxError{} }
