package main

import "github.com/gin-gonic/gin"

func (a *App) phoneSMSLogin(c *gin.Context) { fail(c, 410, "sms_login_disabled") }
