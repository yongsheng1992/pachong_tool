package main

import (
	"fmt"
	"github.com/yongsheng1992/webspiders/spiders/chinadaily"
	"github.com/yongsheng1992/webspiders/spiders/huanqiu"
)

func main() {
	if err := huanqiu.Run(); err != nil {
		fmt.Println(err)
	}
	if err := chinadaily.Run(); err != nil {
		fmt.Println(err)
	}
}
