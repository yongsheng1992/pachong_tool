package chinadaily

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yongsheng1992/webspiders/models"
)

const name = "中国日报网"

func Run(log *logrus.Logger) error {
	dsn := os.Getenv("dsn")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/spiders?charset=utf8mb4&parseTime=True&loc=Local"
	}
	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		return err
	}

	coverPics := map[string]string{}
	c := colly.NewCollector(
		//colly.Debugger(&debug.LogDebugger{}),
		colly.URLFilters(regexp.MustCompile("^https://(\\w+)\\.chinadaily\\.com\\.cn/")),
	)

	c.SetRedirectHandler(func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	})

	if err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*.chinadaily.com.cn*",
		Parallelism: 1,
		Delay:       time.Second * 1,
	}); err != nil {
		return err
	}

	detailCollector := c.Clone()
	imageCollector := c.Clone()

	c.OnHTML("a[href]", func(element *colly.HTMLElement) {
		link := element.Attr("href")
		if strings.HasPrefix(link, "//") {
			link = "https:" + link
		}
		if !strings.HasSuffix(link, ".html") {
			return
		}
		coverPic := element.ChildAttr("img", "src")
		if coverPic != "" {
			if strings.HasPrefix(coverPic, "//") {
				coverPic = "https:" + coverPic
			}
			coverPics[link] = coverPic
			if err := imageCollector.Visit(coverPic); err != nil {
				log.Error(err)
			}
		}

		if err := detailCollector.Visit(link); err != nil {
			log.Error(err)
		}
	})

	imageCollector.OnResponse(func(response *colly.Response) {
		host := response.Request.URL.Host
		path := response.Request.URL.Path
		paths := strings.Split(path, "/")
		filename := paths[len(paths)-1]
		dir := strings.Replace(path, "/"+filename, "", 1)
		imageRoot := "images/"
		_, err := os.Stat(imageRoot + host + dir)
		if err != nil {
			if !os.IsExist(err) {
				if err := os.MkdirAll(imageRoot+host+dir, os.ModePerm); err != nil {
					fmt.Println(err)
				}
			} else {
				panic(err)
			}
		}

		if err := response.Save(fmt.Sprintf("%s%s%s/%s", imageRoot, host, dir, filename)); err != nil {
			log.Error(err)
		}
	})
	detailCollector.OnHTML("div.container", func(element *colly.HTMLElement) {
		log.Error("Crawl ", element.Request.URL.String())
		news := &models.News{
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
		}
		element.ForEach("h1.dabiaoti", func(i int, element *colly.HTMLElement) {
			news.Title = element.Text
		})
		if news.Title == "" {
			return
		}
		element.ForEach("div#Content", func(i int, element *colly.HTMLElement) {
			element.ForEach("img[src]", func(i int, element *colly.HTMLElement) {
				src := element.Attr("src")
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				}
				if err := imageCollector.Visit(src); err != nil {
					fmt.Println(err)
				}
			})
			contentWithTags, _ := element.DOM.Html()
			news.Content = strings.TrimSpace(contentWithTags)
		})
		url := element.Request.URL
		news.OriginLink = fmt.Sprintf("%s://%s%s", url.Scheme, url.Host, url.Path)
		if pic, ok := coverPics[news.OriginLink]; ok {
			news.CoverPic = pic
		}
		news.Source = name
		if err := db.Clauses(clause.OnConflict{
			DoUpdates: clause.AssignmentColumns([]string{"update_time", "content"}),
		}).Create(news).Error; err != nil {
			log.Error(err)
		}
	})
	if err := c.Visit("https://cn.chinadaily.com.cn/"); err != nil {
		return err
	}

	return nil
}
