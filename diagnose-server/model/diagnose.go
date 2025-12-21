package model

type GetDiagnoseListRequest struct {
	ImgLink []string `json:"ImgLink"`
}

type GetDiagnoseListResponse struct {
	Report        Report         `json:"Report"`
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
