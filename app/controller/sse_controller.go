package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/wcs1010270451/helpers/response"
)

type SSEController struct {
}

func (sc *SSEController) Handle(c *gin.Context) {

	response.Success(c)
}
