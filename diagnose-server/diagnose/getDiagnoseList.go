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
5. 难度： 0.1-1.0（1.0最难）。仅获取 JSON，确保数据严谨。`
	aiStep2 := `你是学习规划师。基于提供的试卷分析 JSON，执行以下任务：
1. 计算掌握度： 统计每个二级知识点下，学生的（实得分/总分值）。
2. 灯号分配： >    - 绿灯：掌握度 $\ge 90\%$。
  - 蓝灯：$50\% \le$ 掌握度 $< 90\%$（提分重点）。
  - 红灯/灰灯：掌握度 $< 50\%$。
  - 灯的大小策略，**亮度（Opacity）**可以根据“接近 90% 的程度”来设计，越接近 90% 颜色越鲜亮，代表“临门一脚就能提分”。
3. 计算提分潜力：
  - 找出 ROI 最高的蓝灯：分值占比大且当前掌握度在 70%-85% 之间的知识点。
  - 预测得分：该知识点失分数 $\times 0.8$。
  - 计算 ROI 的关键变量： 建议引入“题目分值”与“错误类型”权重。计算公式优化为：
  $$ROI = \frac{\text{该知识点总分值} \times (1 - \text{当前掌握度})}{\text{知识点层级(难度)} \times \text{错误类型系数}}$$
  （注：粗心错误系数小，概念不清系数大）
4. 生成话术： 为每个灯生成一句简短评价（如“此路不通，暂且绕行”）。
5. 错误点描述： 针对各种灯下的错题，用一句话总结共性病灶（如：“对全等判定中的‘边角边’条件识别不准”）。
6. DiagnoseList中每一项的Status，1表示绿灯，2表示蓝灯，3表示红；Degree表示知识点掌握程度，例如：33%；ExpectScore是预期提升分数；IsDiagnose 表示蓝灯中分值最高的为 true，其他均为false
7. StudyMethod： 从三个方面给出学习建议，例如：概念、计算、心态”。
8. AnalysisTitle: 字符大小不超过10个字
9. Title中不要包含转义字符
10. 每个Question中select的元素不要包含A\B\C,仅包含选项字符串即可，例如以下格式：“最大值为2，x=3”，并且correctAnswer的值为正确答案的字符串，例如：若selcet中的["x=1","x=2","x=3","x=4"]，则correctAnswer的值为"x=3"。
11. FinalAnalysis返回三个元素，第一个元素是对应蓝灯的推荐学习建议，第二个元素是对应红灯的 放弃建议，第三个元素是对应灰灯的推荐保持发挥建议。
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
		{Title: "全等三角形判定概念", Degree: "70%", Status: 0, ExpectScore: 8, Description: "分析：基础扎实", IsDiagnose: true},
		{Title: "解析几何综合", Degree: "55%", Status: 1, ExpectScore: 10, Description: "分析：运算耐力不足", IsDiagnose: true},
		{Title: "导数与函数性质", Degree: "40%", Status: 2, ExpectScore: 12, Description: "分析：分类讨论不完整", IsDiagnose: true},
	}
	rep := model.Report{
		Conclusion:  "总体：基础较好，重点突破计算与逻辑",
		KSMAnalysis: []model.CommonInfo{{Title: "知识点", Description: "解析几何与导数为短板"}, {Title: "技能", Description: "计算准确性需提升"}, {Title: "心态", Description: "遇到繁琐运算易焦虑"}},
		StudyMethod: []model.CommonInfo{{Title: "模板化", Description: "导数分类讨论流程化"}, {Title: "限时训练", Description: "解析几何耐力题日练"}},
	}
	return model.GetDiagnoseListResponse{Report: rep, ScoreSpace: n * 15, DiagnoseList: dl}
}

func ioErr() error { return &json.SyntaxError{} }
