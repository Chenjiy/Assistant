package main

import (
    "bytes"
    "context"
    "encoding/json"
    "github.com/gin-gonic/gin"
    "io"
    "net/http"
    "os"
    "strconv"
    "strings"
)

type DiagnoseInfo struct {
	Title       string `json:"Title"`
	Degree      string `json:"Degree"`
	Status      int64  `json:"Status"`
	ExpectScore int64  `json:"ExpectScore"`
	Description string `json:"Description"`
	IsDiagnose  bool   `json:"isDiagnose"`
}

type Guide struct {
	Title       string `json:"Title"`
	Description string `json:"Description"`
}

type Strategy struct {
	Tille        string `json:"Tille"`
	AbandonInfo  string `json:"AbandonInfo"`
	KeepInfo     string `json:"KeepInfo"`
	OvercomeInfo string `json:"OvercomeInfo"`
}

type Report struct {
	Comments string   `json:"Comments"`
	AllScore int64    `json:"allScore"`
	Guides   []Guide  `json:"Guides"`
	Strategy Strategy `json:"Strategy"`
}

type GetScoreResponse struct {
	ScoreSpace   int64          `json:"ScoreSpace"`
	DiagnoseList []DiagnoseInfo `json:"diagnoseList"`
	Report       Report         `json:"Report"`
}

type Question struct {
	Title         string   `json:"Title"`
	Select        []string `json:"Select"`
	CorrectAnswer string   `json:"CorrectAnswer"`
}

type GetExerciseResponse struct {
	Concepts  string     `json:"Concepts"`
	WarnInfo  string     `json:"WarnInfo"`
	Questions []Question `json:"Questions"`
}

func buildGetScoreResponse(imgs []string) GetScoreResponse {
	n := int64(len(imgs))
	score := n * 15
	dl := []DiagnoseInfo{
		{Title: "全等三角形判定", Degree: "70%", Status: 0, ExpectScore: 8, Description: "分析：基础扎实", IsDiagnose: true},
		{Title: "相似三角形", Degree: "55%", Status: 1, ExpectScore: 6, Description: "分析：概念需巩固", IsDiagnose: true},
		{Title: "勾股定理应用", Degree: "40%", Status: 2, ExpectScore: 10, Description: "分析：计算失误较多", IsDiagnose: false},
	}
	r := Report{
		Comments: "总体评论：已分析" + strconv.FormatInt(n, 10) + "张图片",
		AllScore: score,
		Guides: []Guide{
			{Title: "复习相似与全等", Description: "整理易错点并复盘"},
			{Title: "加强计算能力", Description: "每日10题口算训练"},
		},
		Strategy: Strategy{Tille: "提升策略", AbandonInfo: "避免低效重复练习", KeepInfo: "保留错题整理习惯", OvercomeInfo: "针对薄弱环节专项突破"},
	}
	return GetScoreResponse{ScoreSpace: score, DiagnoseList: dl, Report: r}
}

func buildExerciseResponse(title, desc string) GetExerciseResponse {
	if title == "" {
		title = "全等三角形判定"
	}
	concepts := "概念：" + title
	warn := "注意信息：仔细审题，避免概念混淆"
	qs := []Question{
		{Title: "下列判定中正确的是？", Select: []string{"SSS", "SAS", "ASA", "AAA"}, CorrectAnswer: "SAS"},
		{Title: "两边及夹角相等可判定全等吗？", Select: []string{"可以", "不可以", "取决于角度", "无法判断"}, CorrectAnswer: "可以"},
		{Title: "与题干相关的错误选项是？", Select: []string{"SSA", "ASA", "SSS", "AAS"}, CorrectAnswer: "SSA"},
	}
	return GetExerciseResponse{Concepts: concepts, WarnInfo: warn, Questions: qs}
}

func registerRoutes(r *gin.Engine) {
    r.GET("/getDiagnoseList", func(ctx *gin.Context) {
		raw := ctx.Query("imgLink")
		var imgs []string
		if raw != "" {
			imgs = strings.Split(raw, ",")
		}
		auth := string(ctx.GetHeader("Authorization"))
		appID := "300000281"
		appKey := "2be1698da309b52eb807e9ac2d6a4ff1"
		var out GetScoreResponse
		var ok bool
		if auth != "" || (appID != "" && appKey != "") {
			var err error
			systemInfo := "你是一个诊断高中数学试卷的助手，根据试卷图片链接生成诊断结果。诊断结果格式按照以下结构体返回struct GetScoreResponse {\n    1: i64 ScoreSpace\n    2: list<diagnoseInfo> diagnoseList\n    3: Report report // 核心报告\n}\nstruct DiagnoseInfo {\n    1: string Title // 全等三角形判定\n    2: string Degree // 70%\n    3: i64 Status // 0:绿灯 1：蓝灯 2:红灯\n    4: i64 ExpectScore // 预计增加分数 \n    5: string Description // 分析：基础扎实\n    6: bool isDiagnose // 标记当前是否需要可诊断\n}\nstruct Report {\n    1: string Comments 总体评论\n    2: i64 allScore // 潜力提升空间分数\n    3: list<Guide> Guides // 行动指南\n    4: Strategy Strategy // 策略\n}\nstruct Guide {\n    1: string Title\n    2: string Description\n}\nstruct Strategy {\n    1: string Tille\n    2: string AbandonInfo\n    3: string KeepInfo\n    4: string OvercomeInfo\n}"
			inputInfo := "根据以下图片链接生成诊断结果，必须返回严格JSON，字段为ScoreSpace、diagnoseList、Report，字段名大小写需与示例完全一致。图片链接：" + strings.Join(imgs, ",")
            out, err = callAIForDiagnose(ctx.Request.Context(), imgs, auth, appID, appKey, systemInfo, inputInfo)
			if err == nil {
				ok = true
			}
		}
		if !ok {
			out = buildGetScoreResponse(imgs)
		}
		ctx.JSON(200, out)
	})

r.GET("/getExercise", func(ctx *gin.Context) {
    title := ctx.Query("title")
    desc := ctx.Query("description")
    resp := buildExerciseResponse(title, desc)
    ctx.JSON(200, resp)
})

r.POST("/chat/completions", func(ctx *gin.Context) {
		type ChatMessage struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		type ChatRequest struct {
			Model    string        `json:"model"`
			Messages []ChatMessage `json:"messages"`
		}
    var req ChatRequest
    bdy, _ := ctx.GetRawData()
    if err := json.Unmarshal(bdy, &req); err != nil {
        ctx.JSON(400, map[string]any{"error": "invalid json"})
        return
    }
    auth := ctx.GetHeader("Authorization")
		appID := os.Getenv("TAL_MLOPS_APP_ID")
		appKey := os.Getenv("TAL_MLOPS_APP_KEY")
		if auth == "" && (appID == "" || appKey == "") {
			ctx.JSON(500, map[string]any{"error": "missing credentials"})
			return
		}
		payload, _ := json.Marshal(req)
    upReq, _ := http.NewRequestWithContext(ctx.Request.Context(), "POST", chatAPIEndpoint, bytes.NewReader(payload))
    if auth != "" {
        upReq.Header.Set("Authorization", auth)
    } else {
        upReq.Header.Set("Authorization", "Bearer "+appID+":"+appKey)
    }
    upReq.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(upReq)
		if err != nil {
			ctx.JSON(502, map[string]any{"error": "upstream error"})
			return
		}
		defer resp.Body.Close()
    b, _ := io.ReadAll(resp.Body)
    ctx.Data(200, "application/json", b)
})
}

func main() {
    r := gin.Default()
    registerRoutes(r)
    r.Run(":3000")
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model    string    `json:"model"`
	Messages []chatMsg `json:"messages"`
}

type openaiResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

var chatAPIEndpoint = "http://ai-service.tal.com/openai-compatible/v1/chat/completions"

func callAIForDiagnose(c context.Context, imgs []string, auth, appID, appKey, systemInfo, inputInfo string) (GetScoreResponse, error) {
	req := chatReq{
		Model: "gemini-3-pro",
		Messages: []chatMsg{
			{Role: "system", Content: systemInfo},
			{Role: "user", Content: inputInfo},
		},
	}
	body, _ := json.Marshal(req)
	r, _ := http.NewRequestWithContext(c, "POST", chatAPIEndpoint, bytes.NewReader(body))
	if auth != "" {
		r.Header.Set("Authorization", auth)
	} else {
		r.Header.Set("Authorization", "Bearer "+appID+":"+appKey)
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return GetScoreResponse{}, err
	}
	defer resp.Body.Close()
	var ar openaiResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return GetScoreResponse{}, err
	}
	if len(ar.Choices) == 0 || ar.Choices[0].Message.Content == "" {
		return GetScoreResponse{}, http.ErrBodyNotAllowed
	}
	content := ar.Choices[0].Message.Content
	j := extractJSONFromContent(content)
	type aiResp struct {
		ScoreSpace   int64          `json:"ScoreSpace"`
		DiagnoseList []DiagnoseInfo `json:"diagnoseList"`
		Report       Report         `json:"report"`
	}
	var ai aiResp
	if err := json.Unmarshal([]byte(j), &ai); err != nil {
		return GetScoreResponse{}, err
	}
	return GetScoreResponse{ScoreSpace: ai.ScoreSpace, DiagnoseList: ai.DiagnoseList, Report: ai.Report}, nil
}

func extractJSONFromContent(s string) string {
    if idx := strings.Index(s, "```"); idx >= 0 {
        rest := s[idx+3:]
        if strings.HasPrefix(rest, "json") {
            rest = rest[4:]
        }
        if end := strings.Index(rest, "```"); end >= 0 {
            return strings.TrimSpace(rest[:end])
        }
    }
    start := strings.Index(s, "{")
    end := strings.LastIndex(s, "}")
    if start >= 0 && end >= 0 && end >= start {
        return s[start : end+1]
    }
    return s
}
