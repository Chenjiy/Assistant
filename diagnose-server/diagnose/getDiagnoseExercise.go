package diagnose

import (
	"bytes"
	"diagnose-server/model"
	"diagnose-server/utils"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetDiagnoseExercise(ctx *gin.Context) {
	var req model.GetDiagnoseExerciseRequest
	b, _ := ctx.GetRawData()
	// 解析参数
	if err := json.Unmarshal(b, &req); err != nil {
		fmt.Println("解析请求为json失败：", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
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
	prompt := "根据以下主题与描述生成练习题，必须返回严格JSON。主题：" + title + "；描述：" + desc

	payload := map[string]any{
		"model": "gemini-3-flash",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "你是一个高中数学练习生成助手。请基于主题与描述生成练习数据，并严格按以下结构体返回JSON：\nstruct GetExerciseResponse {\n    1: string Title\n    2: string Concepts\n    3: string WarnInfo\n    4: list<Question> Questions\n}\n\nstruct Question {\n    1: string Title\n    2: list<string> Select\n    3: string CorrectAnswer\n}",
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

func buildDiagnoseExercise(req model.GetDiagnoseExerciseRequest) model.GetExerciseResponse {
	title := req.Title
	if title == "" {
		title = "全等三角形判定概念"
	}
	qs := []model.Question{
		{Title: "下列判定中正确的是？", Select: []string{"SSS", "SAS", "ASA", "AAA"}, CorrectAnswer: "SAS"},
		{Title: "两边及夹角相等可判定全等吗？", Select: []string{"可以", "不可以", "取决于角度", "无法判断"}, CorrectAnswer: "可以"},
		{Title: "关于解析几何的说法正确的是？", Select: []string{"判别式只用于二次方程", "韦达定理可用于系数关系", "抛物线无焦点", "椭圆离心率恒为1"}, CorrectAnswer: "韦达定理可用于系数关系"},
	}
	return model.GetExerciseResponse{Title: title, Concepts: "概念：" + title, WarnInfo: "注意：仔细审题，流程化表达", Questions: qs}
}
