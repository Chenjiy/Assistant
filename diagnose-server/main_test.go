package main

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
    "github.com/gin-gonic/gin"
)

func TestGetDiagnoseList_MockAI(t *testing.T) {
    mockContent := "根据对您提供的试卷图片的识别与分析，以下是诊断：\n" +
        "```json\n" +
        "{\n" +
        "  \"ScoreSpace\": 35,\n" +
        "  \"diagnoseList\": [\n" +
        "    {\"Title\":\"解析几何（圆锥曲线综合）\",\"Degree\":\"60%\",\"Status\":1,\"ExpectScore\":12,\"Description\":\"分析：...\",\"isDiagnose\":true},\n" +
        "    {\"Title\":\"利用导数研究函数性质\",\"Degree\":\"45%\",\"Status\":2,\"ExpectScore\":15,\"Description\":\"分析：...\",\"isDiagnose\":true},\n" +
        "    {\"Title\":\"立体几何（空间向量应用）\",\"Degree\":\"85%\",\"Status\":0,\"ExpectScore\":8,\"Description\":\"分析：...\",\"isDiagnose\":false}\n" +
        "  ],\n" +
        "  \"report\": {\n" +
        "    \"Comments\": \"总体评论...\",\n" +
        "    \"allScore\": 35,\n" +
        "    \"Guides\": [{\"Title\":\"A\",\"Description\":\"B\"}],\n" +
        "    \"Strategy\": {\"Tille\":\"S\",\"AbandonInfo\":\"A\",\"KeepInfo\":\"K\",\"OvercomeInfo\":\"O\"}\n" +
        "  }\n" +
        "}\n" +
        "```\n"
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        resp := map[string]any{
            "choices": []any{map[string]any{"message": map[string]any{"content": mockContent}}},
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    }))
    defer upstream.Close()
    chatAPIEndpoint = upstream.URL

    gin.SetMode(gin.ReleaseMode)
    r := gin.Default()
    registerRoutes(r)
    go r.Run(":3101")
    time.Sleep(300 * time.Millisecond)

    req, _ := http.NewRequest("GET", "http://localhost:3101/getDiagnoseList?imgLink=https://a.jpg,https://b.jpg", nil)
    req.Header.Set("Authorization", "Bearer test:test")
    res, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("request error: %v", err)
    }
    defer res.Body.Close()

    var body GetScoreResponse
    if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
        t.Fatalf("decode error: %v", err)
    }

    if body.ScoreSpace != 35 {
        t.Fatalf("score mismatch: got %d", body.ScoreSpace)
    }
    if len(body.DiagnoseList) != 3 {
        t.Fatalf("diagnoseList size mismatch: got %d", len(body.DiagnoseList))
    }
    if body.Report.AllScore != 35 {
        t.Fatalf("report.allScore mismatch: got %d", body.Report.AllScore)
    }
}
