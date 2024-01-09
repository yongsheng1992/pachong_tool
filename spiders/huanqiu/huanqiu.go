package huanqiu

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

const name = "环球网"

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
			regexp.MustCompile("^https://(\\w+)\\.huanqiu\\.com/"),
			regexp.MustCompile("^https://(\\w+)\\.huanqiucdn\\.cn/"),
		),
	)
	rules := []*colly.LimitRule{
		{
			DomainGlob:  "*.huanqiu.com",
			Parallelism: 1,
			Delay:       time.Second * 1,
		},
		{
			DomainGlob:  "*.huanqiucdn.cn",
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

	detailCollector.OnHTML("html", func(element *colly.HTMLElement) {
		keywords, _ := element.DOM.Find("meta[name='keywords']").Attr("content")

		news := &models.News{
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
			Source:     name,
			Tag:        keywords,
		}

		element.ForEach("textarea", func(i int, element *colly.HTMLElement) {
			class := element.Attr("class")
			switch class {
			case "article-title":
				news.Title = element.Text
			case "article-content":
				contentWithTags, _ := element.DOM.Html()
				query, err := goquery.NewDocumentFromReader(strings.NewReader(html.UnescapeString(contentWithTags)))
				if err != nil {
					return
				}
				query.Find("img").Each(func(i int, selection *goquery.Selection) {
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
					pic = strings.Replace(pic, "https://", "https://{#myhost#}/", 1)
					selection.SetAttr("src", pic)
				})
				contentWithTags, _ = query.Find("article").Html()
				news.Content = strings.TrimSpace(html.UnescapeString(contentWithTags))
			case "article-cover":
				pic := element.Text
				if pic == "" {
					return
				}
				if strings.HasPrefix(pic, "//") {
					pic = "https:" + pic
				}
				if err := imageCollector.Visit(pic); err != nil {
					log.Error(err)
				}
				news.CoverPic = pic
			}
		})

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
	c.OnHTML("a[href]", func(element *colly.HTMLElement) {
		link := element.Attr("href")
		if strings.HasPrefix(link, "//") {
			link = "https:" + link
		}
		match, _ := regexp.MatchString("https://(\\w+)\\.huanqiu\\.com/article/(\\w+)$", link)
		if !match {
			return
		}
		if err := detailCollector.Visit(link); err != nil {
			fmt.Println(err)
		}
	})
	if err := c.Visit("https://www.huanqiu.com/"); err != nil {
		return err
	}
	return nil
}
