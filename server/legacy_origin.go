package main

import (
 "net/url"
 "strings"
 "github.com/gin-gonic/gin"
)

// Preserve old API/media addresses, while normal website visits use HTTPS.
func(a *App)legacyWebsiteOrigin(c *gin.Context){
 if a.Config.Env!="production"||(c.Request.Method!="GET"&&c.Request.Method!="HEAD"){return}
 if strings.HasPrefix(c.Request.URL.Path,"/api/")||strings.HasPrefix(c.Request.URL.Path,"/media/"){return}
 target,err:=url.Parse(a.Config.PublicBaseURL)
 if err!=nil||target.Scheme!="https"||target.Host==""||strings.EqualFold(c.Request.Host,target.Host){return}
 c.Redirect(301,strings.TrimRight(a.Config.PublicBaseURL,"/")+c.Request.URL.RequestURI());c.Abort()
}
