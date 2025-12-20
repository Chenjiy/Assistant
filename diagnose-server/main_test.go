package main

import (
	"encoding/json"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app/server"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetDiagnoseList_MockAI(t *testing.T) {
	mockContent := `{"id":"0a40427669464fa931c0c4dd1f000020","object":"chat.completion","model":"gemini-3-flash","created":1766215593,"system_fingerprint":"","usage":{"prompt_tokens":356,"completion_tokens":862,"total_tokens":2192},"choices":[{"index":0,"message":{"role":"assistant","content":"根据对您提供的试卷图片（涉及解析几何、导数、立体几何等高三数学核心板块）的识别与分析，以下是为您生成的诊断报告：\n\n\<<<json\n{\n  \"ScoreSpace\": 35,\n  \"diagnoseList\": [\n    {\n      \"Title\": \"解析几何（圆锥曲线综合）\",\n      \"Degree\": \"60%\",\n      \"Status\": 1,\n      \"ExpectScore\": 12,\n      \"Description\": \"分析：能够正确列出椭圆\/抛物线方程并尝试联立，但在化简韦达定理及处理弦长、面积最值时存在运算畏难情绪或计算失误。\",\n      \"isDiagnose\": true\n    },\n    {\n      \"Title\": \"利用导数研究函数性质\",\n      \"Degree\": \"45%\",\n      \"Status\": 2,\n      \"ExpectScore\": 15,\n      \"Description\": \"分析：对含参函数的单调性分类讨论逻辑不够严密，特别是在处理隐零点或不等式恒成立问题时，步骤书写不规范，导致丢分严重。\",\n      \"isDiagnose\": true\n    },\n    {\n      \"Title\": \"立体几何（空间向量应用）\",\n      \"Degree\": \"85%\",\n      \"Status\": 0,\n      \"ExpectScore\": 8,\n      \"Description\": \"分析：基础扎实，能熟练建立空间直角坐标系，但在法向量计算的准确性上仍有提升空间。\",\n      \"isDiagnose\": false\n    }\n  ],\n  \"report\": {\n    \"Comments\": \"从卷面看，你的数学基本功较好，中低档难度题目完成度较高。但面对大题（尤其是第20、21题）的复杂运算和深度逻辑推理时，存在“思路断层”和“计算瓶颈”。建议重点攻克解析几何的计算耐力与导数的分类讨论标准化流程。\",\n    \"allScore\": 35,\n    \"Guides\": [\n      {\n        \"Title\": \"解析几何“抗压”训练\",\n        \"Description\": \"针对联立方程、判别式、韦达定理到最终目标的推导过程，进行限时专项练习，重点提升化简技巧（如对称式替换、齐次化处理）。\"\n      },\n      {\n        \"Title\": \"导数模板化表达\",\n        \"Description\": \"总结导数大题的常见讨论点：求导->定定义域->找零点->分区间->列表总结。确保在无法得出最终答案时，逻辑分拿满。\"\n      }\n    ],\n    \"Strategy\": {\n      \"Tille\": \"高考提分策略建议\",\n      \"AbandonInfo\": \"考试中若导数最后一步涉及超纲的超越方程变形，且时间剩余不足10分钟，建议果断放弃，回检选择题填空题。\",\n      \"KeepInfo\": \"稳住立体几何、三角函数及概率统计的满分率，这是保住基本盘的关键。\",\n      \"OvercomeInfo\": \"主攻解析几何第一问及第二问的逻辑架构，尝试攻克导数的第一问分类讨论，这是突破125分+的关键。\"\n    }\n  }\n}\n>>>\n\n### 诊断说明：\n1.  **红灯项（导数）**：图片显示在该板块有较多涂改，说明逻辑思考不连贯，是目前最大的提分突破口。\n2.  **蓝灯项（解析几何）**：属于“会做但做不对”或“做不完”的典型，主要受制于计算繁琐度。\n3.  **绿灯项（立体几何）**：表现稳定，建议继续保持，并在解答题书写中注意“已知、求证、解、答”的规范性。"},"finish_reason":"stop"}]}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": mockContent}}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()
	chatAPIEndpoint = upstream.URL
	fmt.Println("chatAPIEndpoint: ", chatAPIEndpoint)

	h := server.Default(server.WithHostPorts(":3101"))
	registerRoutes(h)
	go h.Spin()
	time.Sleep(300 * time.Millisecond)

	req, _ := http.NewRequest("GET", "http://localhost:3101/getDiagnoseList?imgLink=https://a.jpg,https://b.jpg", nil)
	req.Header.Set("Authorization", "Bearer test:test")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer res.Body.Close()
	var body GetScoreResponse
	var ar openaiResp
	if err := json.NewDecoder(res.Body).Decode(&ar); err != nil {

	}
	if len(ar.Choices) == 0 || ar.Choices[0].Message.Content == "" {

	}
	var content string
	if len(ar.Choices) > 0 {
		content = ar.Choices[0].Message.Content
	}

	j := extractJSONFromContent(content)
	type aiResp struct {
		ScoreSpace   int64          `json:"ScoreSpace"`
		DiagnoseList []DiagnoseInfo `json:"diagnoseList"`
		Report       Report         `json:"report"`
	}
	var ai aiResp
	if err := json.Unmarshal([]byte(j), &ai); err != nil {

	}
	jsonStr, _ := json.Marshal(ai)
	fmt.Println("res.Body: ", string(jsonStr))

	if body.ScoreSpace != 35 {
		t.Fatalf("score mismatch: got %d", body.ScoreSpace)
	}

}
