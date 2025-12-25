package model

type GetDiagnoseListRequest struct {
	ImgLink []string `json:"ImgLink"`
}

type GetDiagnoseListResponse struct {
	ScoreSpace       int64           `json:"ScoreSpace"`       // 为你挖掘到xxx分。【所有蓝灯知识点分值的总和】
	ReportConclusion string          `json:"ReportConclusion"` // 报告：总体评价 【对用户得分及其水平进行整体分析，定位失分最多的知识点，给出对应建议。最后可以建议用户关注蓝灯知识点。】
	AnalysisInfoList []AnalysisInfo  `json:"AnalysisInfoList"` // 分析列表，对应蓝、红、绿的每个知识点以及知识点对应的原题 【蓝灯、红灯、绿灯知识点列表归属至二级知识点展示，展示二级知识点（不超过10字），预计提分值，用户掌握度、对应题号和和题干文本，和分析】
	DeepDiagnoseList []DeepDiagnose  `json:"DeepDiagnoseList"` // 深度诊断
	FinalAnalysis    []FinalAnalysis `json:"FinalAnalysis"`    // 最终分析
}

type AnalysisInfo struct {
	KnowledgeTitle string          `json:"KnowledgeTitle"` // 知识点类型 注：getDiagnoseExercise使用
	Degree         string          `json:"Degree"`         // 知识点掌握程度
	Status         int64           `json:"Status"`         // 知识点掌握度标记 蓝、绿、红
	ExpectScore    int64           `json:"ExpectScore"`    // 预期分数
	Description    string          `json:"Description"`    // 知识点分析 getDiagnoseExercise使用
	Score          string          `json:"Score"`          // 知识点分数
	OriginProblem  []OriginProblem `json:"OriginProblem"`  // 对应原题列表
	IsDiagnose     bool            `json:"isDiagnose"`     // 是否进入诊断详情
}

type OriginProblem struct {
	ProblemTitle  string `json:"ProblemTitle"`  // 原始题目
	ProblemNumber int64  `json:"ProblemNumber"` // 原始题号
}

type DeepDiagnose struct {
	KSMTitle       string   `json:"Title"`         // KSM深度诊断标题
	KSMDescription string   `json:"Description"`   // KSM深度诊断描述
	ProblemNumber  []int64  `json:"ProblemNumber"` // 题号
	Strategy       Strategy `json:"Strategy"`      // 策略
}

type Strategy struct {
	StrategyTitle string `json:"StrategyTitle"` // 策略标题
	StrategyDesp  string `json:"StrategyDesp"`  // 策略描述
}

type FinalAnalysis struct {
	AnalysisTitle string         `json:"AnalysisTitle"`
	AnalysisItem  []AnalysisItem `json:"AnalysisItem"`
}

type AnalysisItem struct {
	AnalysisItemTitle string `json:"AnalysisItemTitle"`
	AnalysisItemDesp  string `json:"AnalysisItemDesp"`
}
