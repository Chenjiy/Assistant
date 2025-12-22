package main

import (
    "diagnose-server/route"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()
    route.RegisterRoutes(r)
    r.Run(":3000")
}

