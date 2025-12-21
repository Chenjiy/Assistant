package route

import (
	"diagnose-server/diagnose"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine) {
	r.POST("/omg/getDiagnoseList", diagnose.GetDiagnoseList)

	r.GET("/omg/getExercise", diagnose.GetDiagnoseExercise)
}
