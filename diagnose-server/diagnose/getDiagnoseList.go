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
	b, _ := ctx.GetRawData()
	if err := json.Unmarshal(b, &req); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json"})
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
	prompt := "根据以下图片链接生成诊断结果，必须返回严格JSON，链接列表：" + strings.Join(imgs, ",")
	aiRoleSet := "你是一个诊断高中数学试卷的助手，根据试卷图片链接生成诊断结果。"
	aiRespSet := "返回结果按照下面的结构体返回, type GetDiagnoseListResponse struct {\n\tReport        Report         `json:\"Report\"`\n\tScoreSpace    int64          `json:\"ScoreSpace\"`\n\tDiagnoseList  []DiagnoseInfo `json:\"DiagnoseList\"`\n\tFinalAnalysis FinalAnalysis  `json:\"FinalAnalysis\"`\n}\n\ntype FinalAnalysis struct {\n\tAnalysisTitle string         `json:\"AnalysisTitle\"`\n\tAnalysisItem  []AnalysisItem `json:\"AnalysisItem\"`\n}\n\ntype AnalysisItem struct {\n\tAnalysisItemTitle string `json:\"AnalysisItemTitle\"`\n\tAnalysisItemDesp  string `json:\"AnalysisItemDesp\"`\n}\n\ntype Report struct {\n\tConclusion  string       `json:\"Conclusion\"`\n\tKSMAnalysis []CommonInfo `json:\"KSMAnalysis\"`\n\tStudyMethod []CommonInfo `json:\"StudyMethod\"`\n}\n\ntype CommonInfo struct {\n\tTitle       string `json:\"Title\"`\n\tDescription string `json:\"Description\"`\n}\n\ntype DiagnoseInfo struct {\n\tTitle       string `json:\"Title\"`\n\tDegree      string `json:\"Degree\"`\n\tStatus      int64  `json:\"Status\"`\n\tExpectScore int64  `json:\"ExpectScore\"`\n\tDescription string `json:\"Description\"`\n\tIsDiagnose  bool   `json:\"IsDiagnose\"`\n}\n\ntype GetDiagnoseExerciseRequest struct {\n\tTitle       string `json:\"Title\"`\n\tDescription string `json:\"Description\"`\n}\n\ntype GetExerciseResponse struct {\n\tTitle     string     `json:\"Title\"`\n\tConcepts  string     `json:\"Concepts\"`\n\tWarnInfo  string     `json:\"WarnInfo\"`\n\tQuestions []Question `json:\"Questions\"`\n}\n\ntype Question struct {\n\tTitle         string   `json:\"Title\"`\n\tSelect        []string `json:\"Select\"`\n\tCorrectAnswer string   `json:\"CorrectAnswer\"`\n}"

	payload := map[string]any{
		"model": "gemini-3-flash",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": aiRoleSet + aiRespSet,
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
