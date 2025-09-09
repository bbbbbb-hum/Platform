package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/wcs1010270451/helpers/response"
)

type HealthController struct {
}

func (hc *HealthController) Handle(c *gin.Context) {
	response.Success(c)
}
