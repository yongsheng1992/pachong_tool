package sohu

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
	"github.com/yongsheng1992/webspiders/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"html"
	"os"
	"regexp"
	"strings"
	"time"
)

const name = "搜狐网"

func Run(log *logrus.Logger) error {
	dsn := os.Getenv("dsn")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/spiders?charset=utf8mb4&parseTime=True&loc=Local"
	}
	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		return err
	}

	c := colly.NewCollector(
		colly.URLFilters(
			regexp.MustCompile("^https://(\\w+)\\.sohu\\.com/"),
			regexp.MustCompile("^https://(\\w+)\\.sohu\\.cn/"),
		),
	)
	rules := []*colly.LimitRule{
		{
			DomainGlob:  "*.sohu.com",
			Parallelism: 1,
			Delay:       time.Second * 1,
		},
		{
			DomainGlob:  "*.sohu.com",
			Parallelism: 1,
			Delay:       time.Second * 1,
		},
	}

	if err := c.Limits(rules); err != nil {
		return err
	}

	detailCollector := c.Clone()
	imageCollector := c.Clone()
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
		fmt.Println("save image ", filename)
		if err := response.Save(fmt.Sprintf("%s%s%s/%s", imageRoot, host, dir, filename)); err != nil {
			fmt.Println(err)
		}
	})

	c.OnHTML("a[href]", func(element *colly.HTMLElement) {
		log.Error("Crawl ", element.Request.URL.String())
		link := element.Attr("href")

		if strings.HasPrefix(link, "//") {
			link = "https:" + link
		}

		if strings.HasPrefix(link, "/a/") {
			link = "https://www.sohu.com" + link
		}
		if !strings.HasPrefix(link, "https://www.sohu.com/a/") {
			return
		}
		if err := detailCollector.Visit(link); err != nil {
			log.Error(err)
		}
	})

	detailCollector.OnHTML("html", func(element *colly.HTMLElement) {
		keywords, _ := element.DOM.Find("meta[name='keywords']").Attr("content")
		news := &models.News{
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
			Source:     name,
			Tag:        keywords,
		}

		title := ""
		element.ForEach("div.text-title h1", func(i int, element *colly.HTMLElement) {
			title = element.Text
		})
		element.ForEach("div.article-title", func(i int, element *colly.HTMLElement) {
			if title == "" {
				title = element.Text
			}
		})

		title = strings.TrimSpace(strings.Trim(title, "\n"))

		content := ""
		element.ForEach("article", func(i int, element *colly.HTMLElement) {
			contentWithTags, _ := element.DOM.Html()
			query, err := goquery.NewDocumentFromReader(strings.NewReader(html.UnescapeString(contentWithTags)))
			if err != nil {
				return
			}
			query.Find("script").Remove()
			query.Find("iframe").Remove()
			query.Find("img").Each(func(_ int, selection *goquery.Selection) {
				pic, ok := selection.Attr("src")
				fmt.Println(pic)
				if !ok || pic == "" {
					return
				}
				if strings.HasPrefix(pic, "//") {
					pic = "https:" + pic
				}
				if err := imageCollector.Visit(pic); err != nil {
					fmt.Println(err)
					return
				}
			})
			content = html.UnescapeString(contentWithTags)
		})
		news.Title = title
		news.Content = strings.TrimSpace(content)

		if news.Title == "" {
			return
		}

		url := element.Request.URL
		news.OriginLink = fmt.Sprintf("%s://%s%s", url.Scheme, url.Host, url.Path)

		if err := db.Clauses(clause.OnConflict{
			DoUpdates: clause.AssignmentColumns([]string{"update_time", "content", "origin_link", "cover_pic"}),
		}).Create(news).Error; err != nil {
			log.Error(err)
		}
	})

	return c.Visit("https://www.sohu.com/")
	//return detailCollector.Visit("https://www.sohu.com/a/744503991_114911?edtsign=C640D0FED68C9296B50B29E91893012628B8F361&edtcode=%2FY42WqEb8rrNuP2rde0SUQ%3D%3D&scm=1103.plate:282:0.0.1_1.0&code=j603pu5s5op")
}
