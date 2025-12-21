package model

type GetDiagnoseListRequest struct {
    ImgLink []string `json:"ImgLink" form:"ImgLink"`
}

type GetDiagnoseListResponse struct {
	Report Report `json:"Report"`
	/* System Prompt:
	你是一名资深中考数学阅卷专家。请分析上传的试卷图片，识别每道题目的以下信息并以 JSON 格式输出：
	1. 题目信息： 题号、题型、分值、正确答案。
	2. 作答情况： 学生答案、得分、正误判断。
	3. 知识点归属： 必须包含 1-5 级知识点（参考中考数学大纲），例如：几何 -> 三角形 -> 全等三角形 -> 全等三角形判定 -> SSS/SAS。
	4. 错误原因分类： [概念模糊, 计算粗心, 逻辑断档, 题目理解偏差, 放弃作答]。
	5. 难度： 0.1-1.0（1.0最难）。
	输出要求： 仅输出 JSON，确保数据严谨。*/
	ScoreSpace    int64          `json:"ScoreSpace"`
	DiagnoseList  []DiagnoseInfo `json:"DiagnoseList"`
	FinalAnalysis FinalAnalysis  `json:"FinalAnalysis"`
}

type FinalAnalysis struct {
	AnalysisTitle string         `json:"AnalysisTitle"`
	AnalysisItem  []AnalysisItem `json:"AnalysisItem"`
}

type AnalysisItem struct {
	AnalysisItemTitle string `json:"AnalysisItemTitle"`
	AnalysisItemDesp  string `json:"AnalysisItemDesp"`
}

type Report struct {
	Conclusion  string       `json:"Conclusion"`
	KSMAnalysis []CommonInfo `json:"KSMAnalysis"`
	StudyMethod []CommonInfo `json:"StudyMethod"`
}

type CommonInfo struct {
	Title       string `json:"Title"`
	Description string `json:"Description"`
}

type DiagnoseInfo struct {
	Title       string `json:"Title"`
	Degree      string `json:"Degree"`
	Status      int64  `json:"Status"`
	ExpectScore int64  `json:"ExpectScore"`
	Description string `json:"Description"`
	IsDiagnose  bool   `json:"IsDiagnose"`
}

type GetDiagnoseExerciseRequest struct {
	Title       string `json:"Title"`
	Description string `json:"Description"`
}

type GetExerciseResponse struct {
	Title     string     `json:"Title"`
	Concepts  string     `json:"Concepts"`
	WarnInfo  string     `json:"WarnInfo"`
	Questions []Question `json:"Questions"`
}

type Question struct {
	Title         string   `json:"Title"`
	Select        []string `json:"Select"`
	CorrectAnswer string   `json:"CorrectAnswer"`
}
