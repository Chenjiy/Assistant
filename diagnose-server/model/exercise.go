package model

// request
type GetDiagnoseExerciseRequest struct {
	Title       string `json:"Title"`
	Description string `json:"Description"`
}

// response
type GetExerciseResponse struct {
	GetExerciseList []GetExerciseList `json:"GetExerciseList"`
}

type GetExerciseList struct {
	Title     string     `json:"Title"`
	Concepts  string     `json:"Concepts"`
	WarnInfo  string     `json:"WarnInfo"`
	Questions []Question `json:"Questions"`
}

type Question struct {
	Title         string   `json:"Title"`
	Select        []Select `json:"Select"`
	CorrectAnswer string   `json:"CorrectAnswer"`
}

type Select struct {
	Parse string
}
