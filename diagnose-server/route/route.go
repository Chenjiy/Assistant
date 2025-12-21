package route

import (
	"diagnose-server/diagnose"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine) {
	r.POST("/omg/getDiagnoseList", diagnose.GetDiagnoseList)

	r.POST("/omg/getExercise", diagnose.GetDiagnoseExercise)
}
