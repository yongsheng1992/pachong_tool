package models

import (
	"time"
)

type News struct {
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Source     string    `json:"source"`
	CoverPic   string    `json:"cover_pic"`
	OriginLink string    `json:"origin_link"`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
	Tag        string    `json:"tag"`
}

//func (news News) TableName() string {
//	return "article"
//}
